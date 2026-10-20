package main

import (
	"math"
	"sync"
	"time"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/audio"
)

// soundMode 是设置页「提示音」的三档。
type soundMode uint8

const (
	// soundOff 不响。
	soundOff soundMode = iota
	// soundSync 只在从已配对设备收到内容时响，是默认档：本机复制不制造噪音。
	soundSync
	// soundAll 本机捕获的新内容也响。
	soundAll
)

// soundModeName 返回档位在设置文件里的写法。默认档不落盘，所以返回空串。
func soundModeName(mode soundMode) string {
	switch mode {
	case soundOff:
		return settings.SoundOff
	case soundAll:
		return settings.SoundAll
	default:
		return ""
	}
}

// soundModeFromName 把设置文件里的写法解析回档位。认不出的取值（旧版本、手改坏的
// 文件）回退到默认档。
func soundModeFromName(name string) soundMode {
	switch name {
	case settings.SoundOff:
		return soundOff
	case settings.SoundAll:
		return soundAll
	default:
		return soundSync
	}
}

// soundPlayer 是提示音的出口。测试注入假实现记录响过几次；生产实现是 chime。
type soundPlayer interface {
	Play()
}

// playArrivalChime 决定一次内容到达要不要响。fromSync 表示这次到达来自已配对
// 设备，而不是本机剪贴板。
//
// 它只看档位，不看时间：响就立刻响，连发几条就是连续几声，不合并也不延迟——
// 提示音的意义是「刚刚进来了东西」，晚一声或并成一声都违背这一点。
func (a *application) playArrivalChime(fromSync bool) {
	switch a.soundMode {
	case soundOff:
		return
	case soundSync:
		if !fromSync {
			return
		}
	}
	if a.sound != nil {
		a.sound.Play()
	}
}

// chime 是提示音的生产实现：第一次响时合成一段短鸣，之后重复播放同一份样本。
//
// 音频设备打不开时 audio 自己按时长静默播完，这里既不读 audio.Err() 也不写日志：
// 提示音失败不该影响内容本身，也不该在日志里留下噪音。
// chimePlayer 是提示音需要的播放器：能停掉上一声就够了。测试靠这个接口观察
// 「上一声真的被停掉了」。
type chimePlayer interface {
	Stop()
}

// chime 是提示音的生产实现：第一次响时合成一段短鸣，之后每次响都从头播一遍。
//
// 新的一声先把上一声停掉，而不是让它们叠在一起：连发几条时听到的是几声清楚
// 分开的响，而不是一团。audio 的 Stop 会把这一声在 5ms 内淡出再丢弃，所以切得
// 干脆，也没有咔哒声。
//
// 音频设备打不开时 audio 自己按时长静默播完，这里既不读 audio.Err() 也不写日志：
// 提示音失败不该影响内容本身，也不该在日志里留下噪音。
type chime struct {
	// newPlayer 造一个立即开始播放的新播放器。
	newPlayer func() chimePlayer
	mu        sync.Mutex
	player    chimePlayer
}

// newChime 返回还没合成的提示音：真正合成推迟到第一次响，不用的用户不付这笔开销。
func newChime() *chime {
	var once sync.Once
	var sound *audio.Sound
	c := &chime{}
	c.newPlayer = func() chimePlayer {
		once.Do(func() {
			sound = audio.NewSound(chimeSampleRate, 1, chimeSamples())
		})
		return sound.Play()
	}
	return c
}

func (c *chime) Play() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.player != nil {
		c.player.Stop()
	}
	c.player = c.newPlayer()
}

const (
	// chimeSampleRate 与 audio 的输出格式一致，省掉一次重采样。
	chimeSampleRate = 48000
	// chimeDuration 是一声的长短：短到连发几条也不会糊成一片，长到听得清。
	chimeDuration = 150 * time.Millisecond
)

// chimeSamples 合成一声单声短鸣：880Hz 主音加一点 1320Hz 的泛音，指数衰减包络，
// 两端各留 5ms 淡入淡出，避免起止的爆音。
func chimeSamples() []float32 {
	const (
		fundamental = 880.0
		overtone    = 1320.0
		// decay 是每秒的衰减指数：0.15 秒后振幅只剩 6.7%，所以尾巴不会拖到下一声里。
		decay = 18.0
	)
	count := int(chimeSampleRate * chimeDuration / time.Second)
	fade := chimeSampleRate / 200 // 5ms
	samples := make([]float32, count)
	for i := range samples {
		t := float64(i) / chimeSampleRate
		value := math.Sin(2*math.Pi*fundamental*t) + 0.3*math.Sin(2*math.Pi*overtone*t)
		value *= math.Exp(-decay * t)
		envelope := 1.0
		if i < fade {
			envelope = float64(i) / float64(fade)
		} else if remaining := count - i; remaining < fade {
			envelope = float64(remaining) / float64(fade)
		}
		samples[i] = float32(0.25 * value * envelope)
	}
	return samples
}

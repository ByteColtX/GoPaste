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

// chimeWindow 是两次提示音之间至少间隔的时间。对方一次连发多条内容、或者「所有
// 新内容」下快速连续复制时，合成一声比一串叮叮叮有用：用户要的是「有东西进来了」。
const chimeWindow = 300 * time.Millisecond

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

// playArrivalChime 决定一次内容到达要不要响，并在响的时候更新合并窗口。
// fromSync 表示这次到达来自已配对设备，而不是本机剪贴板；now 由调用方给出，
// 合并窗口因此可以脱离真实时间测试。
func (a *application) playArrivalChime(fromSync bool, now time.Time) {
	switch a.soundMode {
	case soundOff:
		return
	case soundSync:
		if !fromSync {
			return
		}
	}
	if now.Sub(a.lastChime) < chimeWindow {
		return
	}
	a.lastChime = now
	if a.sound != nil {
		a.sound.Play()
	}
}

// chime 是提示音的生产实现：第一次响时合成一段短鸣，之后重复播放同一份样本。
//
// 音频设备打不开时 audio 自己按时长静默播完，这里既不读 audio.Err() 也不写日志：
// 提示音失败不该影响内容本身，也不该在日志里留下噪音。
type chime struct {
	once  sync.Once
	sound *audio.Sound
}

// newChime 返回还没合成的提示音：真正合成推迟到第一次响，不用的用户不付这笔开销。
func newChime() *chime { return &chime{} }

func (c *chime) Play() {
	c.once.Do(func() {
		c.sound = audio.NewSound(chimeSampleRate, 1, chimeSamples())
	})
	if c.sound != nil {
		c.sound.Play()
	}
}

const (
	// chimeSampleRate 与 audio 的输出格式一致，省掉一次重采样。
	chimeSampleRate = 48000
	// chimeDuration 是一声的长短：短到不拖住注意力，长到听得清。
	chimeDuration = 300 * time.Millisecond
)

// chimeSamples 合成一声单声短鸣：880Hz 主音加一点 1320Hz 的泛音，指数衰减包络，
// 两端各留 5ms 淡入淡出，避免起止的爆音。
func chimeSamples() []float32 {
	const (
		fundamental = 880.0
		overtone    = 1320.0
		// decay 是每秒的衰减指数：0.3 秒后振幅只剩 0.5%，所以尾巴不会拖到下一声里。
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

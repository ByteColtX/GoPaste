package main

import (
	"math"
	"slices"
	"testing"
	"time"

	"gopaste/internal/capture"
	"gopaste/internal/history"
	"gopaste/internal/lansync"
	"gopaste/internal/settings"
)

// textClip 构造一条文字同步内容，测试里用得上。
func textClip(text string) lansync.Clip {
	return lansync.Clip{Kind: history.KindText, Text: text}
}

// recordingSound 是提示音的假出口：只记下响过几次，不出声。
type recordingSound struct{ plays int }

func (r *recordingSound) Play() { r.plays++ }

// TestSoundModeNames 检查档位与设置文件写法互相转换：默认档不落盘，认不出的写法
// 回退到默认。
func TestSoundModeNames(t *testing.T) {
	for _, tc := range []struct {
		mode soundMode
		name string
	}{
		{mode: soundOff, name: settings.SoundOff},
		{mode: soundSync, name: ""},
		{mode: soundAll, name: settings.SoundAll},
	} {
		if got := soundModeName(tc.mode); got != tc.name {
			t.Errorf("soundModeName(%v) = %q，期望 %q", tc.mode, got, tc.name)
		}
		if got := soundModeFromName(tc.name); got != tc.mode {
			t.Errorf("soundModeFromName(%q) = %v，期望 %v", tc.name, got, tc.mode)
		}
	}
	if got := soundModeFromName("loud"); got != soundSync {
		t.Errorf("认不出的写法 = %v，期望回退到 %v", got, soundSync)
	}
}

// TestPlayArrivalChime 检查「响不响」只有一套规则：档位决定哪种到达算数，除此之外
// 一律立刻响，没有合并窗口、也不延迟。
func TestPlayArrivalChime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mode     soundMode
		fromSync bool
		want     int
	}{
		{name: "关闭时同步到达也不响", mode: soundOff, fromSync: true, want: 0},
		{name: "关闭时本机复制也不响", mode: soundOff, fromSync: false, want: 0},
		{name: "仅同步时同步到达响", mode: soundSync, fromSync: true, want: 1},
		{name: "仅同步时本机复制不响", mode: soundSync, fromSync: false, want: 0},
		{name: "所有新内容时本机复制响", mode: soundAll, fromSync: false, want: 1},
		{name: "所有新内容时同步到达也响", mode: soundAll, fromSync: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sound := &recordingSound{}
			app := newApplication()
			app.soundMode = tc.mode
			app.sound = sound

			app.playArrivalChime(tc.fromSync)

			if sound.plays != tc.want {
				t.Errorf("响了 %d 次，期望 %d 次", sound.plays, tc.want)
			}
		})
	}
}

// TestReceiveClipChimesEveryArrival 检查对方连发两条就响两声：每条到达都是独立的
// 「刚进来了东西」，不合并成一声。
func TestReceiveClipChimesEveryArrival(t *testing.T) {
	app, _ := syncApp()
	sound := &recordingSound{}
	app.sound = sound
	app.soundMode = soundSync
	from := remoteDevice("peer-a", "工作室 PC")

	app.receiveClip(textClip("第一条"), from)
	app.receiveClip(textClip("第二条"), from)

	if sound.plays != 2 {
		t.Fatalf("连发两条响了 %d 次，期望 2 次", sound.plays)
	}
}

// TestReceiveClipChimesBeforeHistoryLoads 检查历史还在装载时收到的内容也响：
// 内容已经写进剪贴板，用户感知得到。
func TestReceiveClipChimesBeforeHistoryLoads(t *testing.T) {
	app := newApplication()
	app.clipboard = &recordingClipboard{}
	sound := &recordingSound{}
	app.sound = sound
	app.soundMode = soundSync

	app.receiveClip(textClip("早到的内容"), remoteDevice("peer-a", "工作室 PC"))

	if sound.plays != 1 {
		t.Errorf("响了 %d 次，期望 1 次", sound.plays)
	}
}

// TestAppendCapturedSoundModes 检查本机复制在三种档位下的响法。
func TestAppendCapturedSoundModes(t *testing.T) {
	for _, tc := range []struct {
		mode soundMode
		want int
	}{
		{mode: soundOff, want: 0},
		{mode: soundSync, want: 0},
		{mode: soundAll, want: 1},
	} {
		app, _ := syncApp()
		sound := &recordingSound{}
		app.sound = sound
		app.soundMode = tc.mode

		app.appendCaptured(history.Item{ID: "copy-1", Kind: history.KindText, Text: "复制的文字", CreatedAt: time.Now()})

		if sound.plays != tc.want {
			t.Errorf("档位 %v 下响了 %d 次，期望 %d 次", tc.mode, sound.plays, tc.want)
		}
	}
}

// TestAppendCapturedEchoIsSilent 检查应用自己写进剪贴板的那一次不响：它不是用户
// 复制来的新内容，档位设成「所有新内容」时尤其不能误响。
func TestAppendCapturedEchoIsSilent(t *testing.T) {
	app, _ := syncApp()
	sound := &recordingSound{}
	app.sound = sound
	app.soundMode = soundAll
	// 复制一条富文本记录：剪贴板里只剩它的纯文本，读回来的 ID 因此与原记录不同。
	stored, ok := capture.Snapshot{Kind: kindRichText, Text: "你好", HTML: "<b>你好</b>", At: time.Now()}.Item()
	if !ok {
		t.Fatal("富文本读数没有生成记录")
	}
	app.recordClipboardWrite(stored)
	reading, ok := capture.Snapshot{Kind: kindText, Text: stored.Text, At: time.Now()}.Item()
	if !ok {
		t.Fatal("纯文本读数没有生成记录")
	}

	app.appendCaptured(reading)

	if sound.plays != 0 {
		t.Errorf("回声写入响了 %d 次，期望 0 次", sound.plays)
	}
}

// TestChimeSamples 检查合成的短鸣本身：长度对得上、有声、不削顶，两端淡入淡出。
func TestChimeSamples(t *testing.T) {
	samples := chimeSamples()
	if want := int(chimeSampleRate * chimeDuration / time.Second); len(samples) != want {
		t.Fatalf("样本数 = %d，期望 %d", len(samples), want)
	}
	peak := 0.0
	for _, sample := range samples {
		peak = math.Max(peak, math.Abs(float64(sample)))
		if math.Abs(float64(sample)) > 1 {
			t.Fatalf("样本 %v 超出 [-1, 1]", sample)
		}
	}
	if peak < 0.1 {
		t.Errorf("峰值只有 %v，这一声几乎听不见", peak)
	}
	if first := samples[0]; math.Abs(float64(first)) > 0.001 {
		t.Errorf("第一个样本 = %v，淡入没有从静音开始", first)
	}
	if last := samples[len(samples)-1]; math.Abs(float64(last)) > 0.001 {
		t.Errorf("最后一个样本 = %v，淡出没有收在静音", last)
	}
}

// interruptingPlayer 记下自己被停掉的那一刻，按发生顺序写进共享的事件表。
type interruptingPlayer struct{ events *[]string }

func (p interruptingPlayer) Stop() { *p.events = append(*p.events, "stop") }

// TestChimeInterruptsThePreviousSound 检查新的一声先把上一声停掉再播：连发时听到的
// 是几声分开的响，而不是叠在一起的一团。
func TestChimeInterruptsThePreviousSound(t *testing.T) {
	var events []string
	c := &chime{newPlayer: func() chimePlayer {
		events = append(events, "play")
		return interruptingPlayer{events: &events}
	}}

	c.Play()
	c.Play()

	if want := []string{"play", "stop", "play"}; !slices.Equal(events, want) {
		t.Errorf("事件顺序 = %v，期望 %v", events, want)
	}
}

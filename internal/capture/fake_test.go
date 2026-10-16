package capture

import (
	"context"
	"sync"
)

// fakeSource is a programmable clipboard. Every representation is absent until
// a test sets it, and an error field forces the matching read to fail. It is
// safe for concurrent use because Loop reads on its own goroutine while the
// test reprograms it.
type fakeSource struct {
	mu           sync.Mutex
	sensitive    bool
	sensitiveErr error
	// sensitiveAfterCalls, when positive, makes Sensitive report the marker as
	// set from that call onwards. It lets a test change the answer between the
	// read that precedes the content and the one that follows it.
	sensitiveAfterCalls int
	sensitiveCalls      int
	text                []byte
	textErr             error
	html                []byte
	htmlErr             error
	png                 []byte
	pngErr              error
	files               []string
	filesErr            error

	started   chan struct{}
	startOnce sync.Once
}

func newFakeSource() *fakeSource {
	return &fakeSource{started: make(chan struct{})}
}

// markStarted closes started once the source is first read, so a test can wait
// for Loop to reach the clipboard.
func (s *fakeSource) markStarted() {
	s.startOnce.Do(func() { close(s.started) })
}

func (s *fakeSource) Sensitive(ctx context.Context) (bool, error) {
	s.markStarted()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sensitiveCalls++
	if s.sensitiveAfterCalls > 0 && s.sensitiveCalls >= s.sensitiveAfterCalls {
		return true, nil
	}
	return s.sensitive, s.sensitiveErr
}

func (s *fakeSource) Read(ctx context.Context, rep Representation) ([]byte, error) {
	s.markStarted()
	s.mu.Lock()
	defer s.mu.Unlock()
	switch rep {
	case RepText:
		return payloadOrErr(s.text, s.textErr)
	case RepHTML:
		return payloadOrErr(s.html, s.htmlErr)
	case RepImage:
		return payloadOrErr(s.png, s.pngErr)
	default:
		return nil, ErrNoData
	}
}

func (s *fakeSource) ReadFiles(ctx context.Context) ([]string, error) {
	s.markStarted()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.filesErr != nil {
		return nil, s.filesErr
	}
	if len(s.files) == 0 {
		return nil, ErrNoData
	}
	return append([]string(nil), s.files...), nil
}

func payloadOrErr(data []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrNoData
	}
	return append([]byte(nil), data...), nil
}

func (s *fakeSource) setText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.text = []byte(text)
}

func (s *fakeSource) setHTML(html string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.html = []byte(html)
}

func (s *fakeSource) setImage(png []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.png = append([]byte(nil), png...)
}

func (s *fakeSource) setFiles(paths ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = append([]string(nil), paths...)
}

func (s *fakeSource) setSensitive(sensitive bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sensitive = sensitive
}

func (s *fakeSource) setSensitiveErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sensitiveErr = err
}

func (s *fakeSource) setSensitiveAfter(calls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sensitiveAfterCalls = calls
}

func (s *fakeSource) setTextErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.textErr = err
}

func (s *fakeSource) setImageErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pngErr = err
}

func (s *fakeSource) setFilesErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filesErr = err
}

// fakeTrigger is a change channel a test drives by hand. Watch closes it when
// ctx ends, as the real Trigger contract requires, and close does the same on
// demand so a test can exercise the closed-channel path.
type fakeTrigger struct {
	ch   chan struct{}
	once sync.Once
}

func newFakeTrigger() *fakeTrigger {
	return &fakeTrigger{ch: make(chan struct{}, 16)}
}

func (t *fakeTrigger) Watch(ctx context.Context) <-chan struct{} {
	go func() {
		<-ctx.Done()
		t.close()
	}()
	return t.ch
}

func (t *fakeTrigger) signal() {
	t.ch <- struct{}{}
}

func (t *fakeTrigger) close() {
	t.once.Do(func() { close(t.ch) })
}

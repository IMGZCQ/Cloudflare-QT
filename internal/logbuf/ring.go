// Package logbuf 提供行级环形缓冲，用于限制程序日志的内存占用。
package logbuf

import (
	"bytes"
	"sync"
)

// maxLineBytes 单条日志行的最大字节数，超过则强制截断换行，防止超长行撑爆内存
const maxLineBytes = 64 * 1024

// RingWriter 是一个 io.Writer，仅保留最近 maxLines 行日志。
type RingWriter struct {
	mu    sync.Mutex
	lines []string
	max   int
	cur   []byte // 尚未遇到换行符的残余字节
}

// New 创建一个保留最近 maxLines 行的 RingWriter。
func New(maxLines int) *RingWriter {
	return &RingWriter{max: maxLines}
}

// Write 实现 io.Writer，按换行符拆行入缓冲，超出上限时丢弃最早行。
func (w *RingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cur = append(w.cur, p...)
	for {
		idx := bytes.IndexByte(w.cur, '\n')
		if idx < 0 {
			// 没找到换行：若累计字节已超过单行上限，强制截断为一行，防止恶意/异常输出撑爆内存
			if len(w.cur) > maxLineBytes {
				w.pushLine(string(w.cur[:maxLineBytes]))
				w.cur = w.cur[:0]
			}
			break
		}
		w.pushLine(string(w.cur[:idx]))
		// 用复制的方式前移残余字节，避免底层数组持续增长
		rest := len(w.cur) - idx - 1
		copy(w.cur, w.cur[idx+1:])
		w.cur = w.cur[:rest]
	}
	return len(p), nil
}

// pushLine 追加一行并按上限丢弃最早行；调用前需持有 w.mu
func (w *RingWriter) pushLine(line string) {
	w.lines = append(w.lines, line)
	if len(w.lines) > w.max {
		// 环形裁剪：复制到新切片，避免底层数组无限增长
		kept := make([]string, w.max)
		copy(kept, w.lines[len(w.lines)-w.max:])
		w.lines = kept
	}
}

// Lines 返回当前缓冲区内所有行的副本。
func (w *RingWriter) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

// Package logbuf 提供行级环形缓冲，用于限制程序日志的内存占用。
package logbuf

import (
	"bytes"
	"sync"
)

// maxLineBytes 单条日志行的最大字节数，超过则强制截断换行，防止超长行撑爆内存
const maxLineBytes = 64 * 1024

// RingWriter 是一个 io.Writer，仅保留最近 maxLines 行日志。
// 内部使用环形数组，追加与裁剪均为 O(1)，并维护全局递增序号支持增量读取。
type RingWriter struct {
	mu    sync.Mutex
	lines []string // 环形数组，len == max，head 指向最旧一行
	max   int
	head  int
	count int
	total uint64 // 历史写入总行数（全局递增，不回绕）
	cur   []byte  // 尚未遇到换行符的残余字节
}

// New 创建一个保留最近 maxLines 行的 RingWriter。
func New(maxLines int) *RingWriter {
	return &RingWriter{max: maxLines, lines: make([]string, maxLines)}
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
	w.lines[(w.head+w.count)%w.max] = line
	if w.count < w.max {
		w.count++
	} else {
		w.head = (w.head + 1) % w.max
	}
	w.total++
}

// Lines 返回当前缓冲区内所有行的副本。
func (w *RingWriter) Lines() []string {
	lines, _ := w.LinesFrom(0)
	return lines
}

// LinesFrom 返回序号大于等于 from 的新行，以及当前的总行数。
// from 应传上一次调用返回的 total；当 from 早于已被丢弃的行时，返回当前全部保留行，
// 调用方应依据 total 重新对齐。总行数可用于前端判断是否需要全量刷新。
func (w *RingWriter) LinesFrom(from uint64) ([]string, uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := w.total
	// 缓冲内最早一行的全局序号
	oldest := total - uint64(w.count)
	start := 0
	if from > oldest {
		start = int(from - oldest)
	}
	out := make([]string, w.count-start)
	for i := start; i < w.count; i++ {
		out[i-start] = w.lines[(w.head+i)%w.max]
	}
	return out, total
}

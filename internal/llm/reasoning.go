package llm

import "strings"

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// thinkFilter drops inline `<think>…</think>` blocks from a streamed answer.
//
// `reasoning_effort` asks the model not to reason and a well-behaved proxy puts
// any reasoning in the delta's own `reasoning_content` field, which this package
// never reads. Some gateways do neither and inline the chain of thought in the
// content instead — in a two-sentence bubble that is the whole answer. The
// filter is stateful because a tag can straddle two deltas.
type thinkFilter struct {
	inside bool
	buf    string
}

// feed returns the part of chunk that is safe to show. Text that might still
// turn out to be the start of a tag is held back until the next delta.
func (f *thinkFilter) feed(chunk string) string {
	f.buf += chunk
	var out strings.Builder
	for {
		if f.inside {
			i := strings.Index(f.buf, thinkClose)
			if i < 0 {
				f.buf = partialTail(f.buf, thinkClose)
				break
			}
			f.buf = f.buf[i+len(thinkClose):]
			f.inside = false
			continue
		}
		i := strings.Index(f.buf, thinkOpen)
		if i < 0 {
			held := partialTail(f.buf, thinkOpen)
			out.WriteString(f.buf[:len(f.buf)-len(held)])
			f.buf = held
			break
		}
		out.WriteString(f.buf[:i])
		f.buf = f.buf[i+len(thinkOpen):]
		f.inside = true
	}
	return out.String()
}

// flush releases whatever was held back once the stream ends, so an answer that
// merely trails off in "<thi" is not silently truncated.
func (f *thinkFilter) flush() string {
	if f.inside {
		return ""
	}
	out := f.buf
	f.buf = ""
	return out
}

// partialTail is the longest suffix of s that is a proper prefix of tag.
func partialTail(s, tag string) string {
	max := len(tag) - 1
	if len(s) < max {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasPrefix(tag, s[len(s)-n:]) {
			return s[len(s)-n:]
		}
	}
	return ""
}

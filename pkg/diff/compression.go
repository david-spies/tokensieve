package diff

import (
	"fmt"
	"regexp"
	"strings"
	"tokensieve/pkg/cache"
	"unicode/utf8"
)

// Message mirrors the LLM API message structure.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompressionResult carries the output of a compression pass.
type CompressionResult struct {
	Messages      []Message
	TokensSaved   int64
	BlocksMatched int
	Strategy      string
}

// fileBlockRe matches common agentic file-dump headers used by Claude Code,
// Cursor, Copilot, and similar tools.
var fileBlockRe = regexp.MustCompile(
	`(?m)^(---\s*BEGIN FILE|File:|<file path=|// FILE:|#\s*FILE:)\s*(.+)`,
)

// stackTraceRe matches common stack trace blocks.
var stackTraceRe = regexp.MustCompile(
	`(?s)((?:Error:|Traceback \(most recent call last\)|panic:|goroutine \d+).+?(?:\n\n|\z))`,
)

// CompressContext is the core semantic delta-compression pipeline.
// It applies multiple strategies in order:
//  1. File-block deduplication (high yield, tool-injected context)
//  2. Exact message deduplication (entire message bodies)
//  3. Stack trace deduplication (error log cycling)
//  4. System prompt deduplication
func CompressContext(
	sessionID string,
	messages []Message,
	c *cache.SlidingWindowCache,
) CompressionResult {
	result := CompressionResult{Strategy: "multi-pass"}

	// Pass 1: Whole-message exact dedup
	seen := make(map[string]bool)
	dedupedMessages := make([]Message, 0, len(messages))
	for _, msg := range messages {
		h := c.ComputeHash(msg.Role + "::" + msg.Content)
		if seen[h] {
			result.TokensSaved += estimateTokens(msg.Content)
			result.BlocksMatched++
			continue
		}
		seen[h] = true
		dedupedMessages = append(dedupedMessages, msg)
	}
	messages = dedupedMessages

	// Pass 2: Intra-message semantic block compression
	compressed := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			optimized, saved, matched := compressSystemPrompt(sessionID, msg.Content, c)
			result.TokensSaved += saved
			result.BlocksMatched += matched
			compressed = append(compressed, Message{Role: msg.Role, Content: optimized})
			continue
		}

		if msg.Role == "user" {
			optimized, saved, matched := compressUserMessage(sessionID, msg.Content, c)
			result.TokensSaved += saved
			result.BlocksMatched += matched
			compressed = append(compressed, Message{Role: msg.Role, Content: optimized})
			continue
		}

		compressed = append(compressed, msg)
	}

	result.Messages = compressed
	return result
}

// compressSystemPrompt deduplicates repeated system prompts across sessions.
func compressSystemPrompt(sessionID, content string, c *cache.SlidingWindowCache) (string, int64, int) {
	hash := c.ComputeHash(content)
	if _, found := c.Get(hash); found {
		tokensSaved := estimateTokens(content)
		placeholder := fmt.Sprintf(
			"[TS:SYS_CACHE ref=%s saved≈%dtok]",
			hash[:12], tokensSaved,
		)
		return placeholder, tokensSaved, 1
	}
	c.Put(hash, content, sessionID)
	return content, 0, 0
}

// compressUserMessage applies file-block and stack-trace deduplication
// within a single user message, which is where the bulk of bloat occurs.
func compressUserMessage(sessionID, content string, c *cache.SlidingWindowCache) (string, int64, int) {
	var totalSaved int64
	totalMatched := 0

	// Segment by file blocks
	content, saved, matched := deduplicateFileBlocks(sessionID, content, c)
	totalSaved += saved
	totalMatched += matched

	// Segment stack traces
	content, saved, matched = deduplicateStackTraces(sessionID, content, c)
	totalSaved += saved
	totalMatched += matched

	// Deduplicate any large verbatim paragraphs (>200 chars) as generic blobs
	content, saved, matched = deduplicateLargeBlobs(sessionID, content, c, 200)
	totalSaved += saved
	totalMatched += matched

	return content, totalSaved, totalMatched
}

// deduplicateFileBlocks finds file-dump sections and replaces seen ones
// with compact cache-reference annotations.
func deduplicateFileBlocks(sessionID, content string, c *cache.SlidingWindowCache) (string, int64, int) {
	var saved int64
	matched := 0

	// Split on file headers while preserving the delimiter
	parts := fileBlockRe.Split(content, -1)
	headers := fileBlockRe.FindAllString(content, -1)

	if len(headers) == 0 {
		return content, 0, 0
	}

	var out strings.Builder
	out.WriteString(parts[0]) // pre-header text

	for i, header := range headers {
		var body string
		if i+1 < len(parts) {
			body = parts[i+1]
		}
		block := header + body
		hash := c.ComputeHash(block)

		if entry, found := c.Get(hash); found {
			saved += entry.TokenEst
			matched++
			out.WriteString(fmt.Sprintf(
				"\n[TS:FILE_CACHE ref=%s path=%q saved≈%dtok]\n",
				hash[:12],
				extractPath(header),
				entry.TokenEst,
			))
		} else {
			c.Put(hash, block, sessionID)
			out.WriteString(block)
		}
	}

	return out.String(), saved, matched
}

// deduplicateStackTraces compresses repeated error/panic blocks.
func deduplicateStackTraces(sessionID, content string, c *cache.SlidingWindowCache) (string, int64, int) {
	var saved int64
	matched := 0

	result := stackTraceRe.ReplaceAllStringFunc(content, func(trace string) string {
		hash := c.ComputeHash(trace)
		if entry, found := c.Get(hash); found {
			saved += entry.TokenEst
			matched++
			return fmt.Sprintf("[TS:TRACE_CACHE ref=%s saved≈%dtok]", hash[:12], entry.TokenEst)
		}
		c.Put(hash, trace, sessionID)
		return trace
	})

	return result, saved, matched
}

// deduplicateLargeBlobs hashes contiguous paragraphs above minLen bytes.
func deduplicateLargeBlobs(sessionID, content string, c *cache.SlidingWindowCache, minLen int) (string, int64, int) {
	var saved int64
	matched := 0

	paragraphs := strings.Split(content, "\n\n")
	out := make([]string, 0, len(paragraphs))

	for _, para := range paragraphs {
		if utf8.RuneCountInString(para) < minLen {
			out = append(out, para)
			continue
		}
		hash := c.ComputeHash(para)
		if entry, found := c.Get(hash); found {
			saved += entry.TokenEst
			matched++
			out = append(out, fmt.Sprintf(
				"[TS:BLOB_CACHE ref=%s saved≈%dtok]",
				hash[:12], entry.TokenEst,
			))
		} else {
			c.Put(hash, para, sessionID)
			out = append(out, para)
		}
	}

	return strings.Join(out, "\n\n"), saved, matched
}

// estimateTokens provides a fast approximation: ~4 chars/token (GPT/Claude average).
func estimateTokens(s string) int64 {
	return int64(len(s) / 4)
}

// extractPath pulls the file path from a file-block header line.
func extractPath(header string) string {
	parts := strings.Fields(header)
	if len(parts) > 1 {
		return strings.Trim(parts[len(parts)-1], `"'<>`)
	}
	return "unknown"
}

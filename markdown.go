package tgbotapi

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reKeyboard         = regexp.MustCompile("(?s)```keyboard.*?(```|$)")
	reBlock            = regexp.MustCompile("(?s)```.*?```")
	reInline           = regexp.MustCompile("`[^`\n]+`")
	reHeading          = regexp.MustCompile(`(?m)^#{1,6}\s*(.*)$`)
	reLink             = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)
	reBold             = regexp.MustCompile(`(\\\*\\\*)(?s)(.+?)(\\\*\\\*)`)
	reUnderline        = regexp.MustCompile(`(\\_\\_)(?s)(.+?)(\\_\\_)`)
	reItalicAsterisk   = regexp.MustCompile(`(\\\*)(?s)(.+?)(\\\*)`)
	reItalicUnderscore = regexp.MustCompile(`(\\_)(?s)(.+?)(\\_)`)
	reStrike           = regexp.MustCompile(`\\~\\~(?s)(.+?)\\~\\~`)
	reQuote            = regexp.MustCompile(`(?m)^\\>\s*`)
	reSpoiler          = regexp.MustCompile(`(?s)\|\|(.+?)\|\|`)
)

// Transformer defines an interface for markdown transformations.
type Transformer interface {
	Transform(input string, state *TransformState) string
}

// TransformState holds shared state during the markdown conversion process.
type TransformState struct {
	Blocks []string
}

// MD2V2 converts common Markdown patterns to Telegram's MarkdownV2 format using a series of transformers.
func MD2V2(input string) string {
	state := &TransformState{}
	transformers := []Transformer{
		&KeyboardTransformer{},
		&CodeBlockTransformer{},
		&InlineCodeTransformer{},
		&HeadingTransformer{},
		&LinkTransformer{},
		&SpoilerTransformer{},
		&EscapeTransformer{},
		&StyleTransformer{},
		&RestoreTransformer{},
		&QuoteTransformer{},
	}

	for _, t := range transformers {
		input = t.Transform(input, state)
	}

	return input
}

// EscapeMD2V2 is a helper function to escape text for Telegram's MarkdownV2 parse mode.
func EscapeMD2V2(text string) string {
	return EscapeText(ModeMarkdownV2, text)
}

type KeyboardTransformer struct{}

func (t *KeyboardTransformer) Transform(input string, state *TransformState) string {
	return reKeyboard.ReplaceAllString(input, "")
}

type CodeBlockTransformer struct{}

func (t *CodeBlockTransformer) Transform(input string, state *TransformState) string {
	return reBlock.ReplaceAllStringFunc(input, func(m string) string {
		state.Blocks = append(state.Blocks, m)
		return fmt.Sprintf("\x00BLOCK%d\x00", len(state.Blocks)-1)
	})
}

type InlineCodeTransformer struct{}

func (t *InlineCodeTransformer) Transform(input string, state *TransformState) string {
	return reInline.ReplaceAllStringFunc(input, func(m string) string {
		state.Blocks = append(state.Blocks, m)
		return fmt.Sprintf("\x00BLOCK%d\x00", len(state.Blocks)-1)
	})
}

type HeadingTransformer struct{}

func (t *HeadingTransformer) Transform(input string, state *TransformState) string {
	return reHeading.ReplaceAllString(input, "**$1**")
}

type LinkTransformer struct{}

func (t *LinkTransformer) Transform(input string, state *TransformState) string {
	return reLink.ReplaceAllStringFunc(input, func(m string) string {
		state.Blocks = append(state.Blocks, m)
		return fmt.Sprintf("\x00BLOCK%d\x00", len(state.Blocks)-1)
	})
}

type SpoilerTransformer struct{}

func (t *SpoilerTransformer) Transform(input string, state *TransformState) string {
	return reSpoiler.ReplaceAllStringFunc(input, func(m string) string {
		state.Blocks = append(state.Blocks, m)
		return fmt.Sprintf("\x00BLOCK%d\x00", len(state.Blocks)-1)
	})
}

type EscapeTransformer struct{}

func (t *EscapeTransformer) Transform(input string, state *TransformState) string {
	v2Reserved := []string{"\\", "_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range v2Reserved {
		input = strings.ReplaceAll(input, char, "\\"+char)
	}
	return input
}

type StyleTransformer struct{}

func (t *StyleTransformer) Transform(input string, state *TransformState) string {
	input = reBold.ReplaceAllString(input, "*${2}*")
	input = reUnderline.ReplaceAllString(input, "__${2}__")
	input = reItalicAsterisk.ReplaceAllString(input, "_${2}_")
	input = reItalicUnderscore.ReplaceAllString(input, "_${2}_")
	return reStrike.ReplaceAllString(input, "~${1}~")
}

type RestoreTransformer struct{}

func (t *RestoreTransformer) Transform(input string, state *TransformState) string {
	for i := len(state.Blocks) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("\x00BLOCK%d\x00", i)
		block := state.Blocks[i]
		var restored string
		if strings.HasPrefix(block, "```") {
			content := block[3 : len(block)-3]
			content = strings.ReplaceAll(content, "\\", "\\\\")
			content = strings.ReplaceAll(content, "`", "\\`")

			lang := ""
			if firstLineIdx := strings.Index(content, "\n"); firstLineIdx != -1 {
				lang = strings.TrimSpace(content[:firstLineIdx])
				content = content[firstLineIdx:]
			}

			if lang == "keyboard" {
				restored = ""
			} else {
				restored = "```" + lang + content + "```"
			}
		} else if strings.HasPrefix(block, "`") {
			content := block[1 : len(block)-1]
			content = strings.ReplaceAll(content, "\\", "\\\\")
			content = strings.ReplaceAll(content, "`", "\\`")
			restored = "`" + content + "`"
		} else if strings.HasPrefix(block, "[") {
			match := reLink.FindStringSubmatch(block)
			if len(match) == 3 {
				text := MD2V2(match[1])
				url := match[2]
				url = strings.ReplaceAll(url, "\\", "\\\\")
				url = strings.ReplaceAll(url, ")", "\\)")
				restored = "[" + text + "](" + url + ")"
			}
		} else if strings.HasPrefix(block, "||") {
			match := reSpoiler.FindStringSubmatch(block)
			if len(match) == 2 {
				content := MD2V2(match[1])
				restored = "||" + content + "||"
			}
		} else if strings.HasPrefix(block, "http") {
			// Raw URL
			url := block
			v2Reserved := []string{"\\", "_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
			for _, char := range v2Reserved {
				url = strings.ReplaceAll(url, char, "\\"+char)
			}
			restored = url
		}
		input = strings.ReplaceAll(input, placeholder, restored)
	}
	return input
}

type QuoteTransformer struct{}

func (t *QuoteTransformer) Transform(input string, state *TransformState) string {
	return reQuote.ReplaceAllString(input, "> ")
}

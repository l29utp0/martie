package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"martie/internal/deepseek"
	"martie/internal/localization"
)

const (
	historyMessageRunes = 1000
	historyRuneLimit    = 12000
	participantPrefix   = "@assistant_user_"
)

var participantAliasPattern = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(participantPrefix) + `[a-z0-9_]+`)

type conversationKey struct {
	chatID   int64
	threadID int64
}

type conversation struct {
	exchanges       []exchange
	participants    map[int64]participant
	mentions        map[string]participant
	nextParticipant int
}

type exchange struct {
	userAlias     string
	userText      string
	assistantText string
	createdAt     time.Time
}

type participant struct {
	alias    string
	username string
	display  string
}

func (c *conversation) messages(text localization.Localizer) []deepseek.Message {
	messages := make([]deepseek.Message, 0, len(c.exchanges)*2)
	for _, exchange := range c.exchanges {
		messages = append(messages,
			deepseek.Message{Role: deepseek.RoleUser, Content: formatUserMessage(text, exchange.userAlias, exchange.userText)},
			deepseek.Message{Role: deepseek.RoleAssistant, Content: exchange.assistantText},
		)
	}
	return messages
}

func (c *conversation) userMessage(text localization.Localizer, assistantName string, request assistantRequest) (string, bool) {
	requestText := c.tokenizeUsernames(request.Text)
	if request.ReplyText == "" {
		return requestText, false
	}
	replyText := c.tokenizeUsernames(request.ReplyText)
	if request.ReplyFromBot {
		for _, exchange := range c.exchanges {
			if exchange.assistantText == replyText || c.renderAliases(exchange.assistantText) == request.ReplyText {
				return requestText, false
			}
		}
	}
	replyAuthor := assistantName
	if request.ReplyUserID != 0 {
		replyAuthor = c.participants[request.ReplyUserID].alias
	}
	return text.Format(localization.AssistantReplyContext, "Message being replied to from %s:\n%s\n\nCurrent request:\n%s", replyAuthor, replyText, requestText), true
}

func (c *conversation) expire(now time.Time, ttl time.Duration) int {
	firstCurrent := 0
	for firstCurrent < len(c.exchanges) && now.Sub(c.exchanges[firstCurrent].createdAt) >= ttl {
		firstCurrent++
	}
	c.exchanges = c.exchanges[firstCurrent:]
	return firstCurrent
}

func (c *conversation) remember(userAlias, userText, assistantText string, now time.Time, exchangeLimit int) int {
	c.exchanges = append(c.exchanges, exchange{
		userAlias:     userAlias,
		userText:      truncateRunes(userText, historyMessageRunes),
		assistantText: truncateRunes(assistantText, historyMessageRunes),
		createdAt:     now,
	})
	removed := 0
	for len(c.exchanges) > exchangeLimit || c.runes() > historyRuneLimit {
		c.exchanges = c.exchanges[1:]
		removed++
	}
	return removed
}

func (c *conversation) runes() int {
	total := 0
	for _, exchange := range c.exchanges {
		total += utf8.RuneCountInString(exchange.userText)
		total += utf8.RuneCountInString(exchange.assistantText)
	}
	return total
}

func formatUserMessage(text localization.Localizer, alias, message string) string {
	return text.Format(localization.AssistantUserSays, "Telegram user %s says:\n%s", alias, message)
}

func (c *conversation) participantAlias(userID int64, username, firstName string) string {
	if c.participants == nil {
		c.participants = make(map[int64]participant)
	}
	if c.mentions == nil {
		c.mentions = make(map[string]participant)
	}
	if existing, ok := c.participants[userID]; ok {
		existing.username = username
		existing.display = participantDisplay(username, firstName, existing.alias)
		c.participants[userID] = existing
		return existing.alias
	}
	keyUsername := strings.ToLower(username)
	if mentioned, ok := c.mentions[keyUsername]; username != "" && ok {
		delete(c.mentions, keyUsername)
		mentioned.username = username
		mentioned.display = participantDisplay(username, firstName, mentioned.alias)
		c.participants[userID] = mentioned
		return mentioned.alias
	}
	alias := c.nextAlias()
	c.participants[userID] = participant{
		alias:    alias,
		username: username,
		display:  participantDisplay(username, firstName, alias),
	}
	return alias
}

func (c *conversation) mentionAlias(username string) string {
	for _, participant := range c.participants {
		if strings.EqualFold(participant.username, username) {
			return participant.alias
		}
	}
	if c.mentions == nil {
		c.mentions = make(map[string]participant)
	}
	usernameKey := strings.ToLower(username)
	if mentioned, ok := c.mentions[usernameKey]; ok {
		return mentioned.alias
	}
	alias := c.nextAlias()
	c.mentions[usernameKey] = participant{alias: alias, username: username, display: "@" + username}
	return alias
}

func (c *conversation) nextAlias() string {
	c.nextParticipant++
	return formatParticipantAlias(c.nextParticipant)
}

func formatParticipantAlias(sequence int) string {
	return fmt.Sprintf("%s%04d", participantPrefix, sequence)
}

func participantDisplay(username, firstName, alias string) string {
	if username != "" {
		return "@" + username
	}
	if firstName = safeFirstName(firstName); firstName != "" {
		return firstName
	}
	return neutralAlias(alias)
}

func safeFirstName(name string) string {
	name = strings.TrimSpace(name)
	return strings.Map(func(r rune) rune {
		switch r {
		case '@':
			return '＠'
		case '\n', '\r', '\t':
			return ' '
		default:
			return r
		}
	}, name)
}

func neutralAlias(alias string) string {
	return strings.ReplaceAll(strings.TrimPrefix(alias, "@"), "_", "-")
}

func (c *conversation) tokenizeUsernames(text string) string {
	text = participantAliasPattern.ReplaceAllStringFunc(text, func(alias string) string {
		return neutralAlias(alias)
	})
	for _, participant := range c.participants {
		if participant.username == "" {
			continue
		}
		text = tokenizeUsername(text, participant.username, participant.alias)
	}
	for _, participant := range c.mentions {
		text = tokenizeUsername(text, participant.username, participant.alias)
	}
	return text
}

func tokenizeUsername(text, username, alias string) string {
	mention := regexp.MustCompile(`(?i)(^|[^a-z0-9_])@` + regexp.QuoteMeta(username) + `\b`)
	return mention.ReplaceAllString(text, "${1}"+alias)
}

func (c *conversation) renderAliases(text string) string {
	return participantAliasPattern.ReplaceAllStringFunc(text, func(alias string) string {
		for _, participant := range c.participants {
			if strings.EqualFold(alias, participant.alias) {
				return participant.display
			}
		}
		for _, participant := range c.mentions {
			if strings.EqualFold(alias, participant.alias) {
				return participant.display
			}
		}
		return neutralAlias(alias)
	})
}

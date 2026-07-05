package localization

import "fmt"

type Key string

const (
	AssistantTemporaryFailure  Key = "assistant.temporary_failure"
	AssistantUnexpectedFailure Key = "assistant.unexpected_failure"
	AssistantTooLong           Key = "assistant.too_long"
	AssistantRateLimited       Key = "assistant.rate_limited"
	AssistantFiltered          Key = "assistant.filtered"
	AssistantUserSays          Key = "assistant.user_says"
	AssistantReplyContext      Key = "assistant.reply_context"
	TelegramReplyOne           Key = "telegram.reply_one"
	TelegramReplyMany          Key = "telegram.reply_many"
	TelegramFileOne            Key = "telegram.file_one"
	TelegramFileMany           Key = "telegram.file_many"
	TelegramThreshold          Key = "telegram.threshold"
	TelegramStreamLive         Key = "telegram.stream_live"
)

var portuguesePortugal = map[Key]string{
	AssistantTemporaryFailure:  "Não consegui responder agora.",
	AssistantUnexpectedFailure: "Não consegui responder agora. Pelos vistos, até as máquinas têm dias maus.",
	AssistantTooLong:           "Essa mensagem é demasiado longa. Um pouco de contenção não te mata.",
	AssistantRateLimited:       "Calma. Só consigo aguentar uma certa quantidade de ti de cada vez.",
	AssistantFiltered:          "Não posso ajudar com esse pedido. Sim, até eu tenho limites.",
	AssistantUserSays:          "Utilizador do Telegram %s diz:\n%s",
	AssistantReplyContext:      "Mensagem de %s à qual estás a responder:\n%s\n\nPedido atual:\n%s",
	TelegramReplyOne:           "resposta",
	TelegramReplyMany:          "respostas",
	TelegramFileOne:            "ficheiro",
	TelegramFileMany:           "ficheiros",
	TelegramThreshold:          "chegou a %d em %s",
	TelegramStreamLive:         "🔴 Stream no Miau em direto",
}

type Localizer struct {
	locale Locale
}

func New(locale Locale) Localizer {
	return Localizer{locale: locale}
}

func (l Localizer) Text(key Key, english string) string {
	if l.locale == PortuguesePortugal {
		if translated, ok := portuguesePortugal[key]; ok {
			return translated
		}
	}
	return english
}

func (l Localizer) Format(key Key, english string, args ...any) string {
	return fmt.Sprintf(l.Text(key, english), args...)
}

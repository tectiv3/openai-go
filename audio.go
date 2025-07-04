package openai

import (
	"encoding/json"
	"fmt"
)

// https://platform.openai.com/docs/api-reference/audio

// Transcription struct for response
type Transcription struct {
	CommonResponse

	JSON        *string `json:"json,omitempty"`
	Text        *string `json:"text,omitempty"`
	SRT         *string `json:"srt,omitempty"`
	VerboseJSON *string `json:"verbose_json,omitempty"`
	VTT         *string `json:"vtt,omitempty"`
}

// SpeechVoice type for constants
type SpeechVoice string

const (
	SpeechVoiceAlloy   SpeechVoice = "alloy"
	SpeechVoiceEcho    SpeechVoice = "echo"
	SpeechVoiceFable   SpeechVoice = "fable"
	SpeechVoiceOnyx    SpeechVoice = "onyx"
	SpeechVoiceNova    SpeechVoice = "nova"
	SpeechVoiceShimmer SpeechVoice = "shimmer"
)

// SpeechResponseFormat type for constants
type SpeechResponseFormat string

const (
	SpeechResponseFormatMP3  SpeechResponseFormat = "mp3"
	SpeechResponseFormatOpus SpeechResponseFormat = "opus"
	SpeechResponseFormatAAC  SpeechResponseFormat = "aac"
	SpeechResponseFormatFLAC SpeechResponseFormat = "flac"
)

// SpeechOptions for creating speech
type SpeechOptions map[string]any

// SetResponseFormat sets the `response_format` parameter of speech request.
func (o SpeechOptions) SetResponseFormat(format SpeechResponseFormat) SpeechOptions {
	o["response_format"] = format
	return o
}

// SetSpeed sets the `speed` parameter of speech request.
func (o SpeechOptions) SetSpeed(speed float32) SpeechOptions {
	o["speed"] = speed
	return o
}

// CreateSpeech generates audio from the input text.
//
// https://platform.openai.com/docs/api-reference/audio/createSpeech
func (c *Client) CreateSpeech(model string, input string, voice SpeechVoice, options SpeechOptions) (audio []byte, err error) {
	if options == nil {
		options = SpeechOptions{}
	}
	options["model"] = model
	options["input"] = input
	options["voice"] = voice

	var bytes []byte
	if bytes, err = c.post("audio/speech", options); err == nil {
		return bytes, nil
	} else {
		var res CommonResponse
		if e := json.Unmarshal(bytes, &res); e == nil {
			err = fmt.Errorf("%s: %s", err, res.Error.err())
		}
	}

	return nil, err
}

// TranscriptionResponseFormat type for constants
type TranscriptionResponseFormat string

const (
	TranscriptionResponseFormatJSON        TranscriptionResponseFormat = "json"
	TranscriptionResponseFormatText        TranscriptionResponseFormat = "text"
	TranscriptionResponseFormatSRT         TranscriptionResponseFormat = "srt"
	TranscriptionResponseFormatVerboseJSON TranscriptionResponseFormat = "verbose_json"
	TranscriptionResponseFormatVTT         TranscriptionResponseFormat = "vtt"
)

// TranscriptionOptions for creating transcription
type TranscriptionOptions map[string]any

// SetPrompt sets the `prompt` parameter of transcription request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-prompt
func (o TranscriptionOptions) SetPrompt(prompt string) TranscriptionOptions {
	o["prompt"] = prompt
	return o
}

// SetResponseFormat sets the `response_format` parameter of transcription request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-response_format
func (o TranscriptionOptions) SetResponseFormat(responseFormat TranscriptionResponseFormat) TranscriptionOptions {
	o["response_format"] = responseFormat
	return o
}

// SetTemperature sets the `temperature` parameter of transcription request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-temperature
func (o TranscriptionOptions) SetTemperature(temperature float64) TranscriptionOptions {
	o["temperature"] = temperature
	return o
}

// SetLanguage sets the `language` parameter of transcription request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-language
func (o TranscriptionOptions) SetLanguage(language string) TranscriptionOptions {
	o["language"] = language
	return o
}

// CreateTranscription transcribes given audio file into the input language.
//
// https://platform.openai.com/docs/api-reference/audio/create
func (c *Client) CreateTranscription(file FileParam, model string, options TranscriptionOptions) (response Transcription, err error) {
	if options == nil {
		options = TranscriptionOptions{}
	}
	options["file"] = file
	options["model"] = model

	var bytes []byte
	if bytes, err = c.post("audio/transcriptions", options); err == nil {
		if err = json.Unmarshal(bytes, &response); err == nil {
			if response.Error == nil {
				return response, nil
			}

			err = response.Error.err()
		}
	} else {
		var res CommonResponse
		if e := json.Unmarshal(bytes, &res); e == nil {
			err = fmt.Errorf("%s: %s", err, res.Error.err())
		}
	}

	return Transcription{}, err
}

// Transcription struct for response
type Translation Transcription

// TransclationResponseFormat type for constants
type TranslationResponseFormat TranscriptionResponseFormat

// TranslationOptions for creating transcription
type TranslationOptions map[string]any

// SetPrompt sets the `prompt` parameter of translation request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-prompt
func (o TranslationOptions) SetPrompt(prompt string) TranslationOptions {
	o["prompt"] = prompt
	return o
}

// SetResponseFormat sets the `response_format` parameter of translation request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-response_format
func (o TranslationOptions) SetResponseFormat(responseFormat TranslationResponseFormat) TranslationOptions {
	o["response_format"] = responseFormat
	return o
}

// SetTemperature sets the `temperature` parameter of translation request.
//
// https://platform.openai.com/docs/api-reference/audio/create#audio/create-temperature
func (o TranslationOptions) SetTemperature(temperature float64) TranslationOptions {
	o["temperature"] = temperature
	return o
}

// CreateTranslation translates given audio file into English.
//
// https://platform.openai.com/docs/api-reference/audio/create
func (c *Client) CreateTranslation(file FileParam, model string, options TranslationOptions) (response Translation, err error) {
	if options == nil {
		options = TranslationOptions{}
	}
	options["file"] = file
	options["model"] = model

	var bytes []byte
	if bytes, err = c.post("audio/translations", options); err == nil {
		if err = json.Unmarshal(bytes, &response); err == nil {
			if response.Error == nil {
				return response, nil
			}

			err = response.Error.err()
		}
	} else {
		var res CommonResponse
		if e := json.Unmarshal(bytes, &res); e == nil {
			err = fmt.Errorf("%s: %s", err, res.Error.err())
		}
	}

	return Translation{}, err
}

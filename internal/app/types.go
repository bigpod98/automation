package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type FlexString string

func (f *FlexString) String() string {
	if f == nil {
		return ""
	}
	return string(*f)
}

func (f *FlexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexString(n.String())
		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*f = FlexString(strconv.FormatBool(b))
		return nil
	}

	return fmt.Errorf("value must be string, number, or bool")
}

func (f *FlexString) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		*f = ""
		return nil
	}

	switch node.Kind {
	case yaml.ScalarNode:
		*f = FlexString(strings.TrimSpace(node.Value))
		return nil
	default:
		return fmt.Errorf("value must be scalar")
	}
}

type FlexInt int

func (f *FlexInt) Int() int {
	if f == nil {
		return 0
	}
	return int(*f)
}

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt(i)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("invalid integer string %q", s)
		}
		*f = FlexInt(parsed)
		return nil
	}

	return fmt.Errorf("value must be integer or integer string")
}

func (f *FlexInt) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		*f = 0
		return nil
	}

	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("value must be scalar")
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid integer %q", node.Value)
	}

	*f = FlexInt(parsed)
	return nil
}

type JivedropStandaloneMetadata struct {
	Title   string     `json:"title" yaml:"title"`
	Num     FlexString `json:"num" yaml:"num"`
	Cover   string     `json:"cover" yaml:"cover"`
	Artist  string     `json:"artist" yaml:"artist"`
	Album   string     `json:"album" yaml:"album"`
	Date    string     `json:"date" yaml:"date"`
	Comment string     `json:"comment" yaml:"comment"`
	Stereo  bool       `json:"stereo" yaml:"stereo"`
}

type JivefireStandaloneMetadata struct {
	Episode         FlexInt `json:"episode" yaml:"episode"`
	Title           string  `json:"title" yaml:"title"`
	Channels        FlexInt `json:"channels" yaml:"channels"`
	BarColor        string  `json:"bar_color" yaml:"bar_color"`
	TextColor       string  `json:"text_color" yaml:"text_color"`
	BackgroundImage string  `json:"background_image" yaml:"background_image"`
	ThumbnailImage  string  `json:"thumbnail_image" yaml:"thumbnail_image"`
	NoPreview       bool    `json:"no_preview" yaml:"no_preview"`
	Encoder         string  `json:"encoder" yaml:"encoder"`
	Output          string  `json:"output" yaml:"output"`
}

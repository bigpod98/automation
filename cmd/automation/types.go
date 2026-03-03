package main

import "time"

type config struct {
	Backend       string
	InputDir      string
	InProgressDir string
	FailedDir     string
	PollInterval  time.Duration

	S3Region           string
	S3Bucket           string
	S3InputPrefix      string
	S3InProgressPrefix string
	S3FailedPrefix     string

	AzureAccountURL       string
	AzureContainer        string
	AzureInputPrefix      string
	AzureInProgressPrefix string
	AzureFailedPrefix     string

	JiveTalkingBin string
	JiveFireBin    string
	JiveDropBin    string
}

type jobSpec struct {
	InputAudio    string `json:"input_audio"`
	OutputDir     string `json:"output_dir,omitempty"`
	EpisodeNumber string `json:"episode_number"`
	Title         string `json:"title"`
	ShowTitle     string `json:"show_title,omitempty"`
	CoverArt      string `json:"cover_art"`

	JiveTalking stepSpec       `json:"jivetalking,omitempty"`
	JiveFire    jiveFireSpec   `json:"jivefire,omitempty"`
	JiveDrop    jiveDropSpec   `json:"jivedrop,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type stepSpec struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	ExtraArgs []string `json:"extra_args,omitempty"`
}

type jiveFireSpec struct {
	stepSpec
	OutputPath string   `json:"output_path,omitempty"`
	Channels   int      `json:"channels,omitempty"`
	Encoder    string   `json:"encoder,omitempty"`
	NoPreview  bool     `json:"no_preview,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

type jiveDropSpec struct {
	stepSpec
	OutputPath string   `json:"output_path,omitempty"`
	Artist     string   `json:"artist,omitempty"`
	Album      string   `json:"album,omitempty"`
	Date       string   `json:"date,omitempty"`
	Comment    string   `json:"comment,omitempty"`
	Stereo     bool     `json:"stereo,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
}

type jobResult struct {
	InputAudio      string `json:"input_audio"`
	ProcessedAudio  string `json:"processed_audio"`
	VideoOutput     string `json:"video_output,omitempty"`
	PodcastOutput   string `json:"podcast_output,omitempty"`
	CompletedAtUnix int64  `json:"completed_at_unix"`
}

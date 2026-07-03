package dto

type SeedAudioMetadata struct {
	ClientRequestID string               `json:"client_request_id,omitempty"`
	References      []SeedAudioReference `json:"references,omitempty"`
	SampleRate      *int                 `json:"sample_rate,omitempty"`
	SpeechRate      *float64             `json:"speech_rate,omitempty"`
	LoudnessRate    *float64             `json:"loudness_rate,omitempty"`
	PitchRate       *float64             `json:"pitch_rate,omitempty"`
}

type SeedAudioReference struct {
	Type     string `json:"type,omitempty"`
	URL      string `json:"url,omitempty"`
	AudioURL string `json:"audio_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type SeedAudioResponse struct {
	ID               string                 `json:"id"`
	Object           string                 `json:"object"`
	Created          int64                  `json:"created"`
	Model            string                 `json:"model"`
	URL              string                 `json:"url"`
	URLExpiresAt     int64                  `json:"url_expires_at"`
	Duration         float64                `json:"duration"`
	OriginalDuration float64                `json:"original_duration"`
	Usage            SeedAudioUsage         `json:"usage"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type SeedAudioUsage struct {
	Type             string  `json:"type"`
	Duration         float64 `json:"duration"`
	OriginalDuration float64 `json:"original_duration"`
}

type SeedAudioIdempotencyRecord struct {
	RequestHMAC      string  `json:"request_hmac"`
	Status           string  `json:"status"`
	ResponseID       string  `json:"lsf_audio_id,omitempty"`
	XTTLogID         string  `json:"x_tt_logid,omitempty"`
	TemporaryURL     string  `json:"temporary_url,omitempty"`
	URLExpiresAt     int64   `json:"url_expires_at,omitempty"`
	Duration         float64 `json:"duration,omitempty"`
	OriginalDuration float64 `json:"original_duration,omitempty"`
	ActualQuota      int     `json:"actual_quota,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	ErrorCode        string  `json:"error_code,omitempty"`
	ErrorStatusCode  int     `json:"error_status_code,omitempty"`
	ErrorDiagnostics string  `json:"error_diagnostics,omitempty"`
}

type SeedAudioUpstreamDiagnostics struct {
	UpstreamHTTPStatus        int    `json:"upstream_http_status,omitempty"`
	ContentTypeClass          string `json:"content_type_class,omitempty"`
	XTTLogID                  string `json:"x_tt_logid,omitempty"`
	XTTTraceID                string `json:"x_tt_trace_id,omitempty"`
	RequestID                 string `json:"request_id,omitempty"`
	XRequestID                string `json:"x_request_id,omitempty"`
	ResponseMetadataRequestID string `json:"response_metadata_request_id,omitempty"`
	LatencyMS                 int64  `json:"latency_ms,omitempty"`
	BodySizeBucket            string `json:"body_size_bucket,omitempty"`
	ResponseClass             string `json:"response_class,omitempty"`
	ErrorClass                string `json:"error_class,omitempty"`
	InvalidJSON               bool   `json:"invalid_json,omitempty"`
	HTML                      bool   `json:"html,omitempty"`
	CloudflareLike            bool   `json:"cloudflare_like,omitempty"`
	ReferenceFetchLike        bool   `json:"reference_fetch_like,omitempty"`
}

package config

type Kind int

const (
	KindString Kind = iota
	// KindSecret values are never sent back to the browser.
	KindSecret
	KindBool
	KindInt
	KindFloat
	KindChoice
)

type Choice struct {
	Value string
	Label string
}

// Setting describes one KRILL_ environment variable for the admin panel.
type Setting struct {
	Key         string
	Group       string
	Label       string
	Description string
	Kind        Kind
	Choices     []Choice
	Default     string
	// EnvOnly settings are read before the database is reachable or decide
	// how the server is reached, so the admin panel can't change them.
	EnvOnly bool
}

// Settings are listed in the order the admin panel shows them.
var Settings = []Setting{
	{
		Key:         "KRILL_TEAM_NAME",
		Group:       "General",
		Label:       "Team name",
		Description: "Shown on the sign-in page and in emails.",
		Default:     "Wisconsin Autonomous Underwater Vehicles",
	},
	{
		Key:         "KRILL_UPDATE_CHECK",
		Group:       "General",
		Label:       "Check for updates",
		Description: "Look up the latest release on GitHub.",
		Kind:        KindBool,
		Default:     "true",
	},
	{
		Key:         "KRILL_ALLOW_SIGNUP",
		Group:       "Sign-up",
		Label:       "Open sign-up",
		Description: "Anyone can create a labeler account. The first account can always sign up.",
		Kind:        KindBool,
		Default:     "true",
	},
	{
		Key:   "KRILL_EMAIL_VERIFICATION",
		Group: "Sign-up",
		Label: "Email check",
		Kind:  KindChoice,
		Choices: []Choice{
			{string(VerifyOff), "Any email"},
			{string(VerifyDomain), "Sign-up domain"},
			{string(VerifyCode), "Sign-up domain and emailed code"},
		},
		Default: string(VerifyDomain),
	},
	{
		Key:         "KRILL_SIGNUP_EMAIL_DOMAIN",
		Group:       "Sign-up",
		Label:       "Sign-up domain",
		Description: "Empty allows any domain.",
		Default:     "wisc.edu",
	},
	{
		Key:         "KRILL_RESEND_API_KEY",
		Group:       "Email",
		Label:       "Resend API key",
		Description: "Sends verification, password reset, and sign-in codes.",
		Kind:        KindSecret,
	},
	{
		Key:         "KRILL_EMAIL_FROM",
		Group:       "Email",
		Label:       "From address",
		Description: "At a domain verified in Resend, like Krill <noreply@example.com>.",
	},
	{
		Key:         "KRILL_SLACK_CLIENT_ID",
		Group:       "Slack",
		Label:       "Client ID",
		Description: "Empty turns off Sign in with Slack.",
	},
	{
		Key:   "KRILL_SLACK_CLIENT_SECRET",
		Group: "Slack",
		Label: "Client secret",
		Kind:  KindSecret,
	},
	{
		Key:         "KRILL_SLACK_TEAM_ID",
		Group:       "Slack",
		Label:       "Workspace ID",
		Description: "The only workspace allowed to sign in.",
	},
	{
		Key:         "KRILL_WORKER_TOKEN",
		Group:       "GPU worker",
		Label:       "Worker token",
		Description: "Must match the worker's KRILL_WORKER_TOKEN. Tokens made under Worker tokens work too.",
		Kind:        KindSecret,
	},
	{
		Key:         "KRILL_EMBED_AT_INGEST",
		Group:       "GPU worker",
		Label:       "Embed at ingest",
		Description: "Compute SAM image embeddings for new clips, about 8 MB per frame.",
		Kind:        KindBool,
		Default:     "true",
	},
	{
		Key:         "KRILL_GOLD_REVIEW_THRESHOLD",
		Group:       "Gold review",
		Label:       "Score threshold",
		Description: "Work goes to review while a labeler's recent gold score is below this, from 0 to 1.",
		Kind:        KindFloat,
		Default:     "0.7",
	},
	{
		Key:         "KRILL_GOLD_REVIEW_MIN_ATTEMPTS",
		Group:       "Gold review",
		Label:       "Minimum checks",
		Description: "Gold checks a labeler answers before their score counts.",
		Kind:        KindInt,
		Default:     "3",
	},
	{
		Key:         "KRILL_CLIENT_IP_HEADER",
		Group:       "Network",
		Label:       "Client IP header",
		Description: "Trusted for the client's address, like CF-Connecting-IP. Only set it behind that proxy.",
	},
	{
		Key:         "KRILL_STUCK_CLAIM_DAYS",
		Group:       "Labeling",
		Label:       "Stuck after",
		Description: "Days without edits before a claimed clip is listed as stuck.",
		Kind:        KindInt,
		Default:     "3",
	},
	{Key: "KRILL_PUBLIC_URL", Group: "Server", Label: "Public URL", Default: "http://localhost:8080", EnvOnly: true},
	{Key: "KRILL_API_ADDR", Group: "Server", Label: "Listen address", Default: ":8080", EnvOnly: true},
	{Key: "KRILL_DATABASE_URL", Group: "Server", Label: "Database URL", Kind: KindSecret, EnvOnly: true},
	{Key: "KRILL_WEB_DIR", Group: "Server", Label: "Web app directory", EnvOnly: true},
	{Key: "KRILL_S3_ENDPOINT", Group: "Storage", Label: "Endpoint", Default: "http://localhost:9000", EnvOnly: true},
	{Key: "KRILL_S3_PUBLIC_ENDPOINT", Group: "Storage", Label: "Public endpoint", EnvOnly: true},
	{Key: "KRILL_S3_BUCKET", Group: "Storage", Label: "Bucket", Default: "krill", EnvOnly: true},
	{Key: "KRILL_S3_ACCESS_KEY", Group: "Storage", Label: "Access key", Kind: KindSecret, EnvOnly: true},
	{Key: "KRILL_S3_SECRET_KEY", Group: "Storage", Label: "Secret key", Kind: KindSecret, EnvOnly: true},
	{
		Key:         "KRILL_SAM_MODEL",
		Group:       "GPU worker",
		Label:       "SAM model",
		Description: "Only SAM 3 finds similar objects, and it needs HF_TOKEN on the worker. A worker's own KRILL_SAM_MODEL wins.",
		Kind:        KindChoice,
		Choices: []Choice{
			{"facebook/sam2.1-hiera-tiny", "SAM 2.1 tiny"},
			{"facebook/sam2.1-hiera-small", "SAM 2.1 small"},
			{"facebook/sam2.1-hiera-base-plus", "SAM 2.1 base plus"},
			{"facebook/sam2.1-hiera-large", "SAM 2.1 large"},
			{"facebook/sam3", "SAM 3"},
		},
		Default: "facebook/sam2.1-hiera-small",
	},
}

var (
	byKey    = map[string]Setting{}
	defaults = map[string]string{}
)

func init() {
	for _, s := range Settings {
		byKey[s.Key] = s
		defaults[s.Key] = s.Default
	}
}

func Find(key string) (Setting, bool) {
	s, ok := byKey[key]
	return s, ok
}

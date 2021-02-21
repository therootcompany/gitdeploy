package gitea

// ref
// after
// repository.name
// repository.full_name
// repository.clone_url

// Webhook mirrors https://docs.gitea.io/en-us/webhooks/.
// Created in part with https://mholt.github.io/json-to-go/.
type Webhook struct {
	Secret     string `json:"secret"`
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	CompareURL string `json:"compare_url"`
	Repository struct {
		ID    int `json:"id"`
		Owner struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			FullName  string `json:"full_name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
			Username  string `json:"username"`
		} `json:"owner"`
		Name            string `json:"name"`
		FullName        string `json:"full_name"`
		Description     string `json:"description"`
		Private         bool   `json:"private"`
		Fork            bool   `json:"fork"`
		HTMLURL         string `json:"html_url"`
		SSHURL          string `json:"ssh_url"`
		CloneURL        string `json:"clone_url"`
		Website         string `json:"website"`
		StarsCount      int    `json:"stars_count"`
		ForksCount      int    `json:"forks_count"`
		WatchersCount   int    `json:"watchers_count"`
		OpenIssuesCount int    `json:"open_issues_count"`
		DefaultBranch   string `json:"default_branch"`
		CreatedAt       string `json:"created_at"`
		UpdatedAt       string `json:"updated_at"`
	} `json:"repository"`
}

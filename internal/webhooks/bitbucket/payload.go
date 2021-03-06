package bitbucket

import "time"

// Thank you Matt!
// See https://mholt.github.io/json-to-go/
// See `repo:push payload` on https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/

// Webhook is a smaller version of
type Webhook struct {
	Push       Push       `json:"push"`
	Actor      Actor      `json:"actor"`
	Repository Repository `json:"repository"`
}

// Push is the bitbucket webhook
type Push struct {
	Changes []struct {
		Forced bool `json:"forced"`
		Old    struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Target struct {
				Hash   string `json:"hash"`
				Author struct {
					User struct {
						DisplayName string `json:"display_name"`
						UUID        string `json:"uuid"`
						Nickname    string `json:"nickname"`
						AccountID   string `json:"account_id"`
					} `json:"user"`
				} `json:"author"`
				Date    time.Time `json:"date"`
				Message string    `json:"message"`
				Type    string    `json:"type"`
			} `json:"target"`
		} `json:"old"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		Created bool `json:"created"`
		Commits []struct {
			Rendered struct {
			} `json:"rendered"`
			Hash  string `json:"hash"`
			Links struct {
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
				Comments struct {
					Href string `json:"href"`
				} `json:"comments"`
				Patch struct {
					Href string `json:"href"`
				} `json:"patch"`
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
				Diff struct {
					Href string `json:"href"`
				} `json:"diff"`
				Approve struct {
					Href string `json:"href"`
				} `json:"approve"`
				Statuses struct {
					Href string `json:"href"`
				} `json:"statuses"`
			} `json:"links"`
			Author struct {
				Raw  string `json:"raw"`
				Type string `json:"type"`
				User struct {
					DisplayName string `json:"display_name"`
					UUID        string `json:"uuid"`
					Links       struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
						Avatar struct {
							Href string `json:"href"`
						} `json:"avatar"`
					} `json:"links"`
					Nickname  string `json:"nickname"`
					Type      string `json:"type"`
					AccountID string `json:"account_id"`
				} `json:"user"`
			} `json:"author"`
			Summary struct {
				Raw    string `json:"raw"`
				Markup string `json:"markup"`
				HTML   string `json:"html"`
				Type   string `json:"type"`
			} `json:"summary"`
			Parents []struct {
				Hash  string `json:"hash"`
				Type  string `json:"type"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"parents"`
			Date       time.Time `json:"date"`
			Message    string    `json:"message"`
			Type       string    `json:"type"`
			Properties struct {
			} `json:"properties"`
		} `json:"commits"`
		Truncated bool `json:"truncated"`
		Closed    bool `json:"closed"`
		New       struct {
			Name  string `json:"name"`
			Links struct {
				Commits struct {
					Href string `json:"href"`
				} `json:"commits"`
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			DefaultMergeStrategy string   `json:"default_merge_strategy"`
			MergeStrategies      []string `json:"merge_strategies"`
			Type                 string   `json:"type"`
			Target               struct {
				Rendered struct {
				} `json:"rendered"`
				Hash  string `json:"hash"`
				Links struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
				Author struct {
					Raw  string `json:"raw"`
					Type string `json:"type"`
					User struct {
						DisplayName string `json:"display_name"`
						UUID        string `json:"uuid"`
						Links       struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
							HTML struct {
								Href string `json:"href"`
							} `json:"html"`
							Avatar struct {
								Href string `json:"href"`
							} `json:"avatar"`
						} `json:"links"`
						Nickname  string `json:"nickname"`
						Type      string `json:"type"`
						AccountID string `json:"account_id"`
					} `json:"user"`
				} `json:"author"`
				Summary struct {
					Raw    string `json:"raw"`
					Markup string `json:"markup"`
					HTML   string `json:"html"`
					Type   string `json:"type"`
				} `json:"summary"`
				Parents []struct {
					Hash  string `json:"hash"`
					Type  string `json:"type"`
					Links struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
				} `json:"parents"`
				Date       time.Time `json:"date"`
				Message    string    `json:"message"`
				Type       string    `json:"type"`
				Properties struct {
				} `json:"properties"`
			} `json:"target"`
		} `json:"new"`
	} `json:"changes"`
}

// Actor represents the user / account taking action
type Actor struct {
	DisplayName string `json:"display_name"`
	UUID        string `json:"uuid"`
	Nickname    string `json:"nickname"`
	Type        string `json:"type"`
	AccountID   string `json:"account_id"`
}

// Repository represents repo info
type Repository struct {
	Name    string      `json:"name"`
	Scm     string      `json:"scm"`
	Website interface{} `json:"website"`
	UUID    string      `json:"uuid"`
	Links   struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
		Avatar struct {
			Href string `json:"href"`
		} `json:"avatar"`
	} `json:"links"`
	FullName string `json:"full_name"`
	Owner    struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		Links       struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
		Nickname  string `json:"nickname"`
		Type      string `json:"type"`
		AccountID string `json:"account_id"`
	} `json:"owner"`
	Workspace struct {
		Slug string `json:"slug"`
		Type string `json:"type"`
		Name string `json:"name"`
		UUID string `json:"uuid"`
	} `json:"workspace"`
	Type      string `json:"type"`
	IsPrivate bool   `json:"is_private"`
}

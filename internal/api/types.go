package api

// File represents a Google Drive file or folder.
type File struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MimeType     string   `json:"mimeType"`
	Size         string   `json:"size"`
	CreatedTime  string   `json:"createdTime"`
	ModifiedTime string   `json:"modifiedTime"`
	Parents      []string `json:"parents"`
	Trashed      bool     `json:"trashed"`
	WebViewLink  string   `json:"webViewLink"`
	Starred      bool     `json:"starred"`
	Shared       bool     `json:"shared"`
	OwnedByMe    bool     `json:"ownedByMe"`
	Owners       []User   `json:"owners"`
}

// User represents a Drive user.
type User struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Me           bool   `json:"me"`
}

// Drive represents a Google Shared Drive.
type Drive struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Kind         string             `json:"kind"`
	CreatedTime  string             `json:"createdTime"`
	Capabilities *DriveCapabilities `json:"capabilities"`
}

// DriveCapabilities holds what the current user can do on a shared drive.
type DriveCapabilities struct {
	CanAddChildren   bool `json:"canAddChildren"`
	CanManageMembers bool `json:"canManageMembers"`
	CanShare         bool `json:"canShare"`
	CanDeleteDrive   bool `json:"canDeleteDrive"`
}

// Permission represents an access permission on a file or folder.
type Permission struct {
	ID             string `json:"id"`
	Type           string `json:"type"`  // user, group, domain, anyone
	Role           string `json:"role"`  // owner, organizer, fileOrganizer, writer, commenter, reader
	EmailAddress   string `json:"emailAddress"`
	DisplayName    string `json:"displayName"`
	ExpirationTime string `json:"expirationTime"`
	Deleted        bool   `json:"deleted"`
}

// Change represents a change to a file or shared drive.
type Change struct {
	Kind       string `json:"kind"`
	ChangeType string `json:"changeType"`
	Time       string `json:"time"`
	Removed    bool   `json:"removed"`
	FileID     string `json:"fileId"`
	DriveID    string `json:"driveId"`
	File       *File  `json:"file"`
}

// About contains information about the authenticated user and their quota.
type About struct {
	User         User         `json:"user"`
	StorageQuota StorageQuota `json:"storageQuota"`
}

// StorageQuota holds Drive storage usage information (values are byte counts as strings).
type StorageQuota struct {
	Limit          string `json:"limit"`
	Usage          string `json:"usage"`
	UsageInDrive   string `json:"usageInDrive"`
	UsageInTrash   string `json:"usageInDriveTrash"`
}

// DriveError is returned when the API responds with an error.
type DriveError struct {
	StatusCode int
	Message    string
}

func (e *DriveError) Error() string {
	return e.Message
}

package domain

type ImportAction string

const (
	ImportActionCreate ImportAction = "create"
	ImportActionSkip   ImportAction = "skip"
	ImportActionUpdate ImportAction = "update"
	ImportActionCopy   ImportAction = "copy"
)

type ImportCandidate struct {
	Connection ConnectionInput
	GroupName  string
	Warnings   []string
	ExistingID int64
	Action     ImportAction
	Skipped    bool
}

type ImportPreview struct {
	Candidates []ImportCandidate
	Warnings   []string
}

type ImportSummary struct {
	Created int
	Updated int
	Skipped int
}

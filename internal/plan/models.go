package plan

import "time"

// ChangeType represents the type of change for a resource.
type ChangeType string

const (
	ChangeTypeCreate  ChangeType = "create"
	ChangeTypeUpdate  ChangeType = "update"
	ChangeTypeDelete  ChangeType = "delete"
	ChangeTypeReplace ChangeType = "replace"
	ChangeTypeNoOp    ChangeType = "no-op"
)

// PlanReport is the top-level container for all analyzed plans.
type PlanReport struct {
	Timestamp time.Time
	Stacks    []StackResult
	Summary   PlanSummary
}

// PlanSummary aggregates counts across all stacks.
type PlanSummary struct {
	TotalStacks       int
	StacksWithChanges int
	TotalAdd          int
	TotalChange       int
	TotalDestroy      int
}

// StackResult represents the plan result for a single Terragrunt stack.
type StackResult struct {
	StackPath       string // Relative path from project root
	AbsPath         string // Absolute path
	IsDependency    bool   // True if this stack was run as a dependency
	HasChanges      bool
	ResourceChanges []ResourceChange
	Stats           StackStats
	Error           error // If plan failed
}

// StackStats holds the count of changes for a single stack.
type StackStats struct {
	Add     int
	Change  int
	Destroy int
}

// ResourceChange represents a single resource diff.
type ResourceChange struct {
	Address    string // e.g. aws_instance.web
	Type       string // e.g. aws_instance
	Name       string // e.g. web
	ChangeType ChangeType
	Before     interface{} // JSON structure of resource before change
	After      interface{} // JSON structure of resource after change
	Unknown    interface{} // JSON structure of computed values
}

// TreeNode represents a node in the plan tree (directory or stack).
type TreeNode struct {
	Name       string       // Segment name (e.g., "us-east-1")
	Path       string       // Full relative path
	Stats      StackStats   // Aggregated stats
	HasChanges bool         // True if self or children have changes
	Children   []*TreeNode  // Sub-directories or stacks
	Stack      *StackResult // Nil if directory, set if leaf stack (or mixed)
}

// ProgressMsg represents a progress update during plan collection.
type ProgressMsg struct {
	TotalFiles int    // Total number of files found (only set in initial message)
	Current    int    // Number of files processed so far
	Message    string // Short description, e.g., "Scanning..." or plan path
}

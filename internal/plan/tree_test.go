package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTree(t *testing.T) {
	stacks := []StackResult{
		{
			StackPath:  "dev/us-east-1/vpc",
			HasChanges: true,
			Stats:      StackStats{Add: 1},
		},
		{
			StackPath:  "dev/us-east-1/s3",
			HasChanges: true,
			Stats:      StackStats{Change: 2},
		},
		{
			StackPath:  "dev/eu-west-1/vpc",
			HasChanges: false,
			Stats:      StackStats{},
		},
	}

	roots := BuildTree(stacks)

	// Expected Structure:
	// dev
	//   us-east-1
	//     vpc (Add: 1)
	//     s3 (Change: 2)
	//   eu-west-1
	//     vpc (No Change)

	assert.Len(t, roots, 1)
	dev := roots[0]
	assert.Equal(t, "dev", dev.Name)
	assert.Equal(t, "dev", dev.Path)
	assert.True(t, dev.HasChanges)
	// Stats aggregation: Add: 1, Change: 2 => Add: 1, Change: 2
	assert.Equal(t, 1, dev.Stats.Add)
	assert.Equal(t, 2, dev.Stats.Change)

	assert.Len(t, dev.Children, 2)

	// us-east-1
	// Wait, sortNodes sorts by Name. "eu-west-1" < "us-east-1".
	euWest1 := dev.Children[0]
	usEast1 := dev.Children[1]

	assert.Equal(t, "eu-west-1", euWest1.Name)
	assert.False(t, euWest1.HasChanges)

	assert.Equal(t, "us-east-1", usEast1.Name)
	assert.True(t, usEast1.HasChanges)
	assert.Equal(t, 1, usEast1.Stats.Add)
	assert.Equal(t, 2, usEast1.Stats.Change)

	// s3
	assert.Len(t, usEast1.Children, 2)
	s3 := usEast1.Children[0] // "s3" < "vpc"
	vpc := usEast1.Children[1]

	assert.Equal(t, "s3", s3.Name)
	assert.NotNil(t, s3.Stack)
	assert.Equal(t, 2, s3.Stats.Change)

	assert.Equal(t, "vpc", vpc.Name)
	assert.NotNil(t, vpc.Stack)
	assert.Equal(t, 1, vpc.Stats.Add)
}

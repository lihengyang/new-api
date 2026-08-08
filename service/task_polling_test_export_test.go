package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

// UpdateVideoSingleTaskForTest exposes the production polling step only to the
// external integration-test package, which can import the real Doubao adaptor
// without introducing a service -> adaptor import cycle in production code.
func UpdateVideoSingleTaskForTest(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, taskID string, taskMap map[string]*model.Task) error {
	return updateVideoSingleTask(ctx, adaptor, ch, taskID, taskMap)
}

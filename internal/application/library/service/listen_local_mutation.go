package service

import (
	"hash/fnv"
	"strings"
)

const listenLocalTrackMutationShardCount = 64

func (service *LibraryService) lockListenLocalTrackMutation(fileID string) func() {
	if service == nil {
		return func() {}
	}
	shard := listenLocalTrackMutationShard(fileID)
	service.localTrackMutationMu[shard].Lock()
	return service.localTrackMutationMu[shard].Unlock
}

// lockAllListenLocalTrackMutations is reserved for repository APIs which
// mutate an unknown set of rows atomically (for example DeleteUnavailable).
// The ascending lock order must remain stable to avoid deadlocks.
func (service *LibraryService) lockAllListenLocalTrackMutations() func() {
	if service == nil {
		return func() {}
	}
	for index := range service.localTrackMutationMu {
		service.localTrackMutationMu[index].Lock()
	}
	return func() {
		for index := len(service.localTrackMutationMu) - 1; index >= 0; index-- {
			service.localTrackMutationMu[index].Unlock()
		}
	}
}

func listenLocalTrackMutationShard(fileID string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(fileID)))
	return hasher.Sum32() % listenLocalTrackMutationShardCount
}

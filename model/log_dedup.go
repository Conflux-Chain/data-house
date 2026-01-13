package model

// LogKey defines the composite key for counting logs
// All fields together form the unique identifier for a log entry
type LogKey struct {
	BlockId    uint64 // Block identifier
	TxIndex    uint   // Transaction index within block
	ContractId uint64 // Contract identifier
	TopicId    uint64 // Topic identifier for log classification
	Param1     uint64 // First parameter value
	Param2     uint64 // Second parameter value
	Param3     uint64 // Third parameter value
}

type LogEntry struct {
	Key   *LogKey
	Value *Log
}

// LogCounter manages in-memory counting of log entries
// Uses composite key for grouping and counting duplicate log entries
type LogCounter struct {
	// count map stores the occurrence count for each unique log combination
	// Key is the composite LogKey, value is the count of occurrences
	count map[LogKey]uint
}

// NewLogCounter creates and initializes a new LogCounter instance
// Returns a ready-to-use counter with initialized internal map
func NewLogCounter() *LogCounter {
	return &LogCounter{
		count: make(map[LogKey]uint),
	}
}

// IncrementAndGet increases the count for the given log and returns the new count
// Creates a composite key from log fields and updates the counter
func (lc *LogCounter) IncrementAndGet(log *Log) (uint, *LogKey) {
	key := LogKey{
		TxIndex:    log.TxIndex,
		ContractId: log.ContractId,
		TopicId:    log.TopicId,
		Param1:     log.Param1,
		Param2:     log.Param2,
		Param3:     log.Param3,
	}

	// Increment count for this key
	lc.count[key]++
	return lc.count[key], &key
}

// GetCount retrieves the current count for the given log without modifying it
// Returns 0 if the log combination has not been seen before
func (lc *LogCounter) GetCount(key *LogKey) uint {
	return lc.count[*key]
}

// Reset clears all counting data from the counter
// Re-initializes the internal map to empty state
func (lc *LogCounter) Reset() {
	lc.count = make(map[LogKey]uint)
}

// GetTotalCount returns the total number of unique log combinations counted
// This represents the size of the internal map
func (lc *LogCounter) GetTotalCount() int {
	return len(lc.count)
}

// ProcessAndCountLogs processes a slice of logs and returns a new slice with Count fields populated
// Each log's Count field is set based on its occurrence order in the sequence
func (lc *LogCounter) ProcessAndCountLogs(logs []*Log) []*Log {
	result := make([]*Log, len(logs))

	for i, log := range logs {
		// Create a copy of the log to avoid modifying the original
		newLog := *log
		// Set the Count field based on occurrence
		newLog.Count, _ = lc.IncrementAndGet(log)
		result[i] = &newLog
	}

	return result
}

// GetStats returns statistics about the counted logs
// Includes total unique combinations and maximum count value
func (lc *LogCounter) GetStats() (uniqueCombinations int, maxCount uint) {
	uniqueCombinations = len(lc.count)

	for _, count := range lc.count {
		if count > maxCount {
			maxCount = count
		}
	}

	return uniqueCombinations, maxCount
}

// GetAllCounts returns a copy of all counts for debugging or analysis
// Useful for testing or monitoring purposes
func (lc *LogCounter) GetAllCounts() map[LogKey]uint {
	// Create a copy to prevent external modification
	counts := make(map[LogKey]uint, len(lc.count))
	for key, value := range lc.count {
		counts[key] = value
	}
	return counts
}

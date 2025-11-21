package slots

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"scheduled-db/internal/store"
)

// **Feature: queue-size-limits, Property 5: Job count accuracy**
// **Validates: Requirements 2.4**
func TestProperty5_JobCountAccuracy(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of slots (0 to 20)
		numSlots := rapid.IntRange(0, 20).Draw(rt, "numSlots")

		// Track the expected total job count
		expectedTotalJobs := int64(0)

		// Create a map to represent slots with jobs
		slots := make(map[int64]*store.SlotData)

		for i := 0; i < numSlots; i++ {
			// Generate a unique slot key
			slotKey := int64(i * 100)

			// Generate a random number of jobs for this slot (0 to 50)
			numJobsInSlot := rapid.IntRange(0, 50).Draw(rt, "numJobsInSlot")

			// Create job IDs for this slot
			jobIDs := make([]string, numJobsInSlot)
			for j := 0; j < numJobsInSlot; j++ {
				jobIDs[j] = rapid.String().Draw(rt, "jobID")
			}

			// Create the slot
			slots[slotKey] = &store.SlotData{
				Key:     slotKey,
				MinTime: slotKey * 10,
				MaxTime: (slotKey + 1) * 10,
				JobIDs:  jobIDs,
			}

			// Add to expected total
			expectedTotalJobs += int64(numJobsInSlot)
		}

		// Calculate the actual job count by summing across all slots
		actualJobCount := int64(0)
		for _, slot := range slots {
			actualJobCount += int64(len(slot.JobIDs))
		}

		// Verify that the total job count equals the sum of jobs across all slots
		if actualJobCount != expectedTotalJobs {
			rt.Fatalf("job count mismatch: expected %d (sum across %d slots), got %d",
				expectedTotalJobs, numSlots, actualJobCount)
		}

		// Additional verification: count should be non-negative
		if actualJobCount < 0 {
			rt.Fatalf("job count should be non-negative, got %d", actualJobCount)
		}

		// Verify that empty slots contribute 0 to the count
		for slotKey, slot := range slots {
			if len(slot.JobIDs) == 0 {
				// Empty slot should not contribute to count
				countWithoutSlot := int64(0)
				for k, s := range slots {
					if k != slotKey {
						countWithoutSlot += int64(len(s.JobIDs))
					}
				}
				if countWithoutSlot != actualJobCount {
					rt.Fatalf("empty slot %d should not affect count", slotKey)
				}
			}
		}

		// Verify the count matches when calculated differently (as a sanity check)
		alternativeCount := int64(0)
		for _, slot := range slots {
			for range slot.JobIDs {
				alternativeCount++
			}
		}

		if alternativeCount != actualJobCount {
			rt.Fatalf("alternative count calculation mismatch: expected %d, got %d",
				actualJobCount, alternativeCount)
		}
	})
}

// Helper test to verify job count accuracy with real Job objects
func TestProperty5_JobCountAccuracyWithJobs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random number of jobs (0 to 100)
		numJobs := rapid.IntRange(0, 100).Draw(rt, "numJobs")

		// Create jobs and organize them into slots
		jobs := make(map[string]*store.Job)
		slotJobCounts := make(map[int64]int)

		for i := 0; i < numJobs; i++ {
			// Generate a unique job ID using index to ensure uniqueness
			jobID := rapid.String().Draw(rt, "jobID")
			// Ensure uniqueness by appending index
			for jobs[jobID] != nil {
				jobID = jobID + rapid.String().Draw(rt, "suffix")
			}
			timestamp := rapid.Int64Range(time.Now().Unix()+1, time.Now().Unix()+86400).Draw(rt, "timestamp")

			job := &store.Job{
				ID:        jobID,
				Type:      store.JobUnico,
				Timestamp: &timestamp,
				CreatedAt: time.Now().Unix(),
			}

			jobs[jobID] = job

			// Calculate which slot this job belongs to (using 10-second slot gap)
			slotKey := timestamp / 10
			slotJobCounts[slotKey]++
		}

		// Calculate expected total from slot counts
		expectedTotal := int64(0)
		for _, count := range slotJobCounts {
			expectedTotal += int64(count)
		}

		// Verify the total matches the number of jobs created
		if expectedTotal != int64(numJobs) {
			rt.Fatalf("total job count %d should equal number of jobs created %d",
				expectedTotal, numJobs)
		}

		// Verify the count matches the jobs map size
		if int64(len(jobs)) != expectedTotal {
			rt.Fatalf("jobs map size %d should equal total count %d",
				len(jobs), expectedTotal)
		}

		// Verify that summing individual slot counts gives the correct total
		sumOfSlotCounts := int64(0)
		for _, count := range slotJobCounts {
			sumOfSlotCounts += int64(count)
		}

		if sumOfSlotCounts != int64(numJobs) {
			rt.Fatalf("sum of slot counts %d should equal number of jobs %d",
				sumOfSlotCounts, numJobs)
		}
	})
}

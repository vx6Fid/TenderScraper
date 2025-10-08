package utils

// CalculateOptimalWorkers determines concurrency based on job count
func CalculateOptimalWorkers(totalJobs int) int {
	if totalJobs < 100 {
		return max(1, totalJobs/10)
	}

	workers := totalJobs / 10
	if workers < 10 {
		workers = 10
	}
	if workers > 120 {
		workers = 120
	}
	return workers
}

func CalculateWorkersPastLinks(totalJobs int) int {
	return min(60, totalJobs)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package helm

import (
	batch "k8s.io/api/batch/v1"
)

type JobStatus int

const (
	JobStatusSuccess int = iota
	JobStatusFailed
	JobStatusProgressing
)

// DetermineJobStatus determines the status of the job based on job status fields
func DetermineJobStatus(job *batch.Job) int {
	if job.Status.CompletionTime != nil && job.Status.Succeeded > 0 {
		return JobStatusSuccess
	} else if job.Status.StartTime != nil && job.Status.Failed > 0 {
		return JobStatusFailed
	}
	return JobStatusProgressing
}

package limiter

// AdmissionResult describes how a caller proceeded through the limiter.
type AdmissionResult int

const (
	// AdmissionAcquired means the caller acquired real limiter capacity.
	AdmissionAcquired AdmissionResult = iota + 1
	// AdmissionBypassed means the caller proceeded without consuming limiter capacity.
	AdmissionBypassed
)

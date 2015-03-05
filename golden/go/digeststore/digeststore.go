package digeststore

// DigestInfo aggregates all information we have about an individual digest.
type DigestInfo struct {
	// TestName for this digest.
	TestName string

	// Digest that uniquely identifies the digest within this test.
	Digest string

	// First containes the timestamp of the first occurance of this digest.
	First int64

	// Last contains the timestamp of the last time we have seen this digest.
	Last int64

	// Exception stores a string representing the exception that was encountered
	// retrieving this digest. If Exception is "" then there was no problem.
	Exception string
}

type DigestStore interface {
	GetDigestInfo(testName, digest string) *DigestInfo
}

// TODO(stephana): Implement a caching digest store.

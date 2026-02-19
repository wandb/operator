package strimzi

// Purge (for when WBRetentionPolicy's OnDelete == WBPurgeOnDelete) is
// handled via `translator/v2/kafka.go` with the KafkaNodePool's
// Spec->Storage->Volumes->DeleteClaim == true
//
// No custom logic is required.

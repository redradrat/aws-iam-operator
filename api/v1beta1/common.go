package v1beta1

type SyncState string

const (
	SyncSyncState  SyncState = "SYNC"
	OkSyncState    SyncState = "OK"
	ErrorSyncState SyncState = "ERROR"
)

type AWSObjectStatus struct {

	// +kubebuilder:validation:optional
	//
	// State holds the current state of the resource
	State SyncState `json:"state"`

	// +kubebuilder:validation:optional
	//
	// Message holds the current/last status message from the operator.
	Message string `json:"message"`

	// +kubebuilder:validation:optional
	//
	// LastSyncTime holds the timestamp of the last sync attempt
	LastSyncAttempt string `json:"lastSyncAttempt"`

	// +kubebuilder:validation:optional
	//
	// Arn holds the concrete AWS ARN of the managed policy
	ARN string `json:"arn"`

	// +kubebuilder:validation:optional
	//
	// ObservedGeneration holds the generation (metadata.generation in CR) observed by the controller
	ObservedGeneration int64 `json:"observedGeneration"`
}

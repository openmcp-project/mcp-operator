package errors

import "fmt"

var ErrWrongComponentConfigType = fmt.Errorf("the given configuration has the wrong type for this component's spec")
var ErrWrongComponentStatusType = fmt.Errorf("the given status has the wrong type for this component's status field in the ManagedControlPlane")

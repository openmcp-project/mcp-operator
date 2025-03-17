package components

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AnnotationAlreadyExistsError struct {
	Annotation   string
	DesiredValue string
	ActualValue  string
}

func NewAnnotationAlreadyExistsError(ann, desired, actual string) *AnnotationAlreadyExistsError {
	return &AnnotationAlreadyExistsError{
		Annotation:   ann,
		DesiredValue: desired,
		ActualValue:  actual,
	}
}

func (e *AnnotationAlreadyExistsError) Error() string {
	return fmt.Sprintf("annotation '%s' already exists on the object and value '%s' could not be updated to '%s'", e.Annotation, e.ActualValue, e.DesiredValue)
}

func IsAnnotationAlreadyExistsError(err error) bool {
	_, ok := err.(*AnnotationAlreadyExistsError)
	return ok
}

// PatchAnnotation patches the given annotation into the given object.
// Returns a AnnotationAlreadyExistsError if the annotation exists with a different value on the object and mode ANNOTATION_OVERWRITE is not set.
// To remove an annotation, set mode to ANNOTATION_DELETE. The given annValue does not matter in this case.
// Note that mode is meant to be a single optional mode argument. The behavior if multiple modes are specified at the same time is undefined.
func PatchAnnotation(ctx context.Context, c client.Client, obj client.Object, annKey, annValue string, mode ...PatchAnnotationMode) error {
	modeDelete := false
	modeOverwrite := false
	for _, m := range mode {
		switch m {
		case ANNOTATION_DELETE:
			modeDelete = true
		case ANNOTATION_OVERWRITE:
			modeOverwrite = true
		}
	}
	quote := "\""
	anns := obj.GetAnnotations()
	if anns == nil {
		anns = map[string]string{}
	}
	val, ok := anns[annKey]
	if ok {
		if !modeDelete {
			if val == annValue {
				// annotation already exists on the object, nothing to do
				return nil
			}
			if !modeOverwrite {
				return NewAnnotationAlreadyExistsError(annKey, annValue, val)
			}
		} else {
			// delete annotation
			annValue = "null"
			quote = ""
		}
	} else {
		if modeDelete {
			// annotation does not exist, nothing to do
			return nil
		}
	}
	if err := c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":%s%s%s}}}`, annKey, quote, annValue, quote)))); err != nil {
		return err
	}
	return nil
}

type PatchAnnotationMode string

const (
	ANNOTATION_OVERWRITE PatchAnnotationMode = "overwrite"
	ANNOTATION_DELETE    PatchAnnotationMode = "delete"
)

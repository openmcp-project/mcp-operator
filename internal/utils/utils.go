package utils

import (
	"context"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	maxLength                int = 63
	Base32EncodeStdLowerCase     = "abcdefghijklmnopqrstuvwxyz234567"
)

// K8sNameHash takes any number of string arguments and computes a hash out of it, which is then base32-encoded to be a valid k8s resource name.
// The arguments are joined with '/' before being hashed.
func K8sNameHash(ids ...string) string {
	name := strings.Join(ids, "/")
	h := sha1.New()
	_, _ = h.Write([]byte(name))
	// we need base32 encoding as some base64 (even url safe base64) characters are not supported by k8s
	// see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
	return base32.NewEncoding(Base32EncodeStdLowerCase).WithPadding(base32.NoPadding).EncodeToString(h.Sum(nil))
}

// ScopeToControlPlane is a convenience function which wraps K8sNameHash(cpMeta.Namespace, cpMeta.Name, ids...).
func ScopeToControlPlane(cpMeta *metav1.ObjectMeta, ids ...string) string {
	return K8sNameHash(append([]string{cpMeta.Namespace, cpMeta.Name}, ids...)...)
}

// IsNil checks if a given pointer is nil.
// Opposed to 'i == nil', this works for typed and untyped nil values.
func IsNil(i any) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

// InitializeControllerLogger initializes a new logger.
// Panics if the context doesn't already contain a logger.
// The given name is added to the logger (it's supposed to be the controller's name).
// Returns the logger and the context containing it.
func InitializeControllerLogger(ctx context.Context, name string) (logging.Logger, context.Context) {
	log := logging.FromContextOrPanic(ctx).WithName(name)
	ctx = logging.NewContext(ctx, log)
	return log, ctx
}

// PrefixWithNamespace prefixes the given name with the sourceNamespace.
func PrefixWithNamespace(sourceNamespace, name string) string {
	if len(sourceNamespace) == 0 {
		sourceNamespace = "default"
	}

	return shorten(fmt.Sprintf("%s--%s", sourceNamespace, name), maxLength)
}

// shorten shortens the given string to the provided length
func shorten(input string, maxLength int) string {
	if len(input) <= maxLength {
		return input
	}

	hash := fnv.New32a()
	hash.Write([]byte(input))

	suffix := fmt.Sprintf("--%x", hash.Sum32())
	trimLength := maxLength - len(suffix)

	return input[:trimLength] + suffix
}

// Package check runs the checks of an item version: the package check,
// which every version gets, then the check its format names. A check only
// reads the package; nothing in it is ever run.
package check

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/plenigon/plenior-hub/internal/communities"
)

// Package is the name of the check every version gets.
const Package = "package"

// MaxMessageBytes is the longest message the service takes.
const MaxMessageBytes = 1000

// maxUnpackedBytes bounds what a gzip package may unpack to, so a small
// package cannot fill the runner.
const maxUnpackedBytes = 1 << 30

// Func is the check of a format. It reads job and pkg, the package already
// proven to be the one job names, and answers passed or failed with a
// message for the item's owner.
type Func func(ctx context.Context, job communities.Job, pkg []byte) (status, message string)

// Registry holds the format checks this hub runs, by the name a format
// registers.
type Registry map[string]Func

// Run checks pkg, the package of job, and returns the result of each
// check. fetchErr is why the package could not be proven to be the one job
// names, if it could not. The format's own check always passes or fails:
// a check this hub does not have, or that cannot run, fails.
func Run(ctx context.Context, job communities.Job, pkg []byte, fetchErr error, reg Registry) []communities.Result {
	status, message := communities.Failed, ""
	if fetchErr != nil {
		message = fetchErr.Error()
	} else {
		status, message = inspect(job.MediaType, pkg)
	}
	out := []communities.Result{result(Package, status, message)}
	if job.Check == Package {
		return out
	}
	if status != communities.Passed {
		return append(out, result(job.Check, communities.Failed,
			"The package check failed, so the "+job.Check+" check did not run."))
	}
	fn, ok := reg[job.Check]
	if !ok {
		return append(out, result(job.Check, communities.Failed, "This hub has no check named "+job.Check+"."))
	}
	status, message = guarded(ctx, fn, job, pkg)
	if status != communities.Passed && status != communities.Failed {
		status, message = communities.Failed, fmt.Sprintf("The %s check answered %q, not passed or failed.", job.Check, status)
	}
	return append(out, result(job.Check, status, message))
}

// guarded runs fn, failing the check when it panics.
func guarded(ctx context.Context, fn Func, job communities.Job, pkg []byte) (status, message string) {
	defer func() {
		if r := recover(); r != nil {
			status, message = communities.Failed, "The "+job.Check+" check stopped on an error of the hub."
		}
	}()
	return fn(ctx, job, pkg)
}

// inspect checks that pkg reads as its media type. A type the hub does not
// know passes on its size and SHA-256 alone.
func inspect(mediaType string, pkg []byte) (status, message string) {
	switch mediaType {
	case "application/zip":
		if _, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg))); err != nil {
			return communities.Failed, "The package is not a zip: " + err.Error() + "."
		}
	case "application/gzip":
		r, err := gzip.NewReader(bytes.NewReader(pkg))
		if err != nil {
			return communities.Failed, "The package is not a gzip stream: " + err.Error() + "."
		}
		n, err := io.Copy(io.Discard, io.LimitReader(r, maxUnpackedBytes+1))
		switch {
		case err != nil:
			return communities.Failed, "The gzip stream is broken: " + err.Error() + "."
		case n > maxUnpackedBytes:
			return communities.Failed, "The package unpacks to more than 1 GiB."
		}
	case "application/json":
		if !json.Valid(pkg) {
			return communities.Failed, "The package is not valid JSON."
		}
	default:
		return communities.Passed, "The hub does not read " + mediaType + "; only the size and the SHA-256 were checked."
	}
	return communities.Passed, ""
}

// result returns a result whose message fits the service.
func result(name, status, message string) communities.Result {
	if len(message) > MaxMessageBytes {
		message = message[:MaxMessageBytes]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	return communities.Result{Name: name, Status: status, Message: message}
}

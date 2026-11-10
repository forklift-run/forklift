package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	fplt "github.com/forklift-run/forklift/exp/pallets"
	"github.com/forklift-run/forklift/exp/staging"
	"github.com/forklift-run/forklift/internal/app/forklift"
	"github.com/forklift-run/forklift/internal/clients/docker"
)

// Download

func DownloadImagesForStoreApply(
	ctx context.Context, indent int,
	store *staging.FSStageStore, platform, toolVersion, bundleMinVersion string,
	parallel, ignoreToolVersion bool,
) error {
	next, hasNext := store.GetNext()
	current, hasCurrent := store.GetCurrent()
	indent++

	if hasCurrent && current != next {
		bundle, err := store.LoadFSBundle(current)
		if err != nil {
			return errors.Wrapf(err, "couldn't load staged pallet bundle %d", current)
		}
		if err = CheckBundleShallowCompat(
			bundle, toolVersion, bundleMinVersion, ignoreToolVersion,
		); err != nil {
			return err
		}
		IndentedFprintln(
			indent, os.Stderr,
			"Downloading Docker container images specified by the last successfully-applied staged "+
				"pallet bundle, in case the next to be applied fails to be applied...",
		)
		if err := DownloadImages(
			ctx, indent+1, bundle, bundle, platform, false, parallel, false,
		); err != nil {
			return err
		}
	}
	if hasNext {
		bundle, err := store.LoadFSBundle(next)
		if err != nil {
			return errors.Wrapf(err, "couldn't load staged pallet bundle %d", next)
		}
		if err = CheckBundleShallowCompat(
			bundle, toolVersion, bundleMinVersion, ignoreToolVersion,
		); err != nil {
			return err
		}
		IndentedFprintln(
			indent, os.Stderr,
			"Downloading Docker container images specified by the next staged pallet bundle to be "+
				"applied...",
		)
		if err := DownloadImages(
			ctx, indent+1, bundle, bundle, platform, false, parallel, false,
		); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func DownloadImages(
	ctx context.Context, indent int,
	deplsLoader ResolvedDeplsLoader, pkgLoader fplt.FSPkgLoader,
	platform string, includeDisabled, parallel, forcePull bool,
) error {
	orderedImages, err := forklift.ListRequiredImages(deplsLoader, pkgLoader, includeDisabled)
	if err != nil {
		return errors.Wrap(err, "couldn't determine images required by package deployments")
	}
	if len(orderedImages) == 0 {
		// When there are no images to download, don't cause an error if we can't initialize the
		// Docker API client!
		return nil
	}

	dc, err := docker.NewClient()
	if err != nil {
		return errors.Wrap(err, "couldn't make Docker API client")
	}

	if !parallel {
		return downloadImagesSerial(ctx, indent, orderedImages, platform, dc, forcePull)
	}
	return downloadImagesParallel(ctx, indent, orderedImages, platform, dc, forcePull)
}

func downloadImagesSerial(
	ctx context.Context,
	indent int, images []string, platform string, dc *docker.Client, forcePull bool,
) error {
	p := newProgress(len(images))
	for _, image := range images {
		unpinned, digest := splitImage(image)
		mustPull := true
		if digest == "" {
			IndentedFprintf(indent, os.Stderr, "%s Downloading %s...\n", p.Render(p.Get()), image)
		} else {
			IndentedFprintf(indent, os.Stderr, "%s Downloading %s...\n", p.Render(p.Get()), unpinned)
			IndentedFprintf(indent+1, os.Stderr, "Full image:")
			IndentedFprintf(p.RenderWidth()/len(indentation), os.Stderr, " %s\n", image)
			if _, err := dc.InspectImage(ctx, image); err == nil {
				IndentedFprintln(
					indent+1, os.Stderr, "Digest-pinned image appears to have been downloaded already",
				)
				mustPull = forcePull
			}
		}

		if mustPull {
			out := docker.NewOutStream(os.Stderr)
			_, isTerminal := term.GetFdInfo(os.Stderr)
			if err := dc.PullImage(
				ctx, image, platform, out,
				jsonmessage.WithMessagePrinter(indentedMessagePrinter(indent+1, isTerminal, 0)),
			); err != nil {
				return errors.Wrapf(err, "couldn't download %s", image)
			}
			IndentedFprintln(indent+1, os.Stderr, "Finished download")
		}

		p.Increment()
	}
	return nil
}

func splitImage(image string) (unpinned, digest string) {
	unpinned, digest, ok := strings.Cut(image, "@sha256:")
	if !ok {
		return image, ""
	}
	return unpinned, digest
}

type progress struct {
	max int

	lock  *sync.RWMutex
	value int
}

func newProgress(max int) *progress {
	return &progress{
		max:  max,
		lock: &sync.RWMutex{},
	}
}

func (p *progress) Increment() (result int) {
	if p.lock != nil {
		p.lock.Lock()
		defer p.lock.Unlock()
	}
	p.value++
	return p.value
}

func (p *progress) Get() (result int) {
	if p.lock != nil {
		p.lock.RLock()
		defer p.lock.RUnlock()
	}
	return p.value
}

func (p *progress) Render(value int) string {
	maxWidth := len(fmt.Sprintf("%d", p.max))
	return fmt.Sprintf(fmt.Sprintf("[%%%dd/%d]", maxWidth, p.max), value)
}

func (p *progress) RenderWidth() int {
	maxWidth := len(fmt.Sprintf("%d", p.max))
	return 1 + maxWidth + 1 + maxWidth + 1
}

func indentedMessagePrinter(
	indent int, isTerminal bool, width uint16,
) func(jsonstream.Message, io.Writer) error {
	// This function is adapted from the displayJSONMessages function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0. The function was changed to adapt the default
	// terminal-aware presentation for use with jsonmessage.WithMessagePrinter, and to replace
	// jsonmessage.Display with indentedDisplayMessage.
	ids := make(map[string]uint)
	return func(jm jsonstream.Message, out io.Writer) error {
		var diff uint
		if jm.ID == "" || jm.Progress == nil {
			// When outputting something that isn't progress output, clear the history of previous lines.
			// We don't want progress entries from some previous operation to be updated (for example,
			// pull -a with multiple tags).
			ids = make(map[string]uint)
		} else {
			line, ok := ids[jm.ID]
			if !ok {
				// NOTE: This approach of using len(id) to figure out the number of lines of history only
				// works as long as we clear the history when we output something that's not accounted for
				// in the map, such as a line with no ID.
				line = uint(len(ids))
				ids[jm.ID] = line
				if isTerminal {
					_, _ = fmt.Fprintf(out, "\n")
				}
			}
			diff = uint(len(ids)) - line
			if isTerminal {
				cursorUp(out, diff)
			}
		}
		err := indentedDisplayMessage(indent, jm, out, isTerminal, width)
		if jm.ID != "" && isTerminal {
			cursorDown(out, diff)
		}
		return err
	}
}

func indentedDisplayMessage(
	indent int, jm jsonstream.Message, out io.Writer, isTerminal bool, width uint16,
) error {
	// This function is adapted from the Display function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0. This function was changed to use IndentedFprintf instead
	// of fmt.Fprintf.
	if jm.Error != nil {
		return jm.Error
	}
	var endl string
	if isTerminal && jm.Stream == "" && jm.Progress != nil {
		clearLine(out)
		endl = "\r"
		IndentedFprint(indent, out, endl)
	} else if jm.Progress != nil && (jm.Progress.Current > 0 || jm.Progress.Total > 0) {
		// disable progressbar in non-terminal
		return nil
	}
	if jm.ID != "" {
		IndentedFprintf(indent, out, "%s: ", jm.ID)
	}
	switch {
	case jm.Progress != nil && isTerminal:
		if width == 0 {
			const defaultWidth = 200
			width = defaultWidth
		}
		IndentedFprintf(
			indent, out, "%s %s%s", jm.Status, jsonmessage.RenderTUIProgress(*jm.Progress, width), endl,
		)
	case jm.Stream != "":
		IndentedFprintf(indent, out, "%s%s", jm.Stream, endl)
	default:
		IndentedFprintf(indent, out, "%s%s\n", jm.Status, endl)
	}
	return nil
}

func clearLine(out io.Writer) {
	// This function is adapted from the clearLine function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0.
	const ansiEraseLine = "\x1b[2K" // Erase entire line
	_, _ = out.Write([]byte(ansiEraseLine))
}

func cursorUp(out io.Writer, l uint) {
	// This function is adapted from the cursorUp function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0.
	if l == 0 {
		return
	}

	const ansiCursorUpFmt = "\x1b[%dA" // Move cursor up N lines
	_, _ = fmt.Fprintf(out, ansiCursorUpFmt, l)
}

func cursorDown(out io.Writer, l uint) {
	// This function is adapted from the cursorDown function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0.
	if l == 0 {
		return
	}

	const ansiCursorDownFmt = "\x1b[%dB" // Move cursor down N lines
	_, _ = fmt.Fprintf(out, ansiCursorDownFmt, l)
}

func downloadImagesParallel(
	ctx context.Context,
	indent int, images []string, platform string, dc *docker.Client, forcePull bool,
) error {
	eg, egctx := errgroup.WithContext(ctx)
	p := newProgress(len(images))
	m := &parallelPullMuxer{lock: &sync.Mutex{}}
	pl := &sync.Mutex{}
	for _, image := range images {
		eg.Go(func() error {
			unpinned, digest := splitImage(image)
			headerImage := image
			if digest != "" {
				headerImage = unpinned
			}
			batcher := newMessageBatcher(m, pl, func() string {
				return fmt.Sprintf("%s Downloading %s...", p.Render(p.Get()), headerImage)
			})
			mustPull := true
			if digest != "" {
				buf := bytes.Buffer{}
				fmt.Fprintf(&buf, "Full image:")
				IndentedFprintf(p.RenderWidth()/len(indentation), &buf, " %s\n", image)
				batcher.Add(jsonstream.Message{Stream: buf.String()})
				if _, err := dc.InspectImage(ctx, image); err == nil {
					batcher.Add(jsonstream.Message{
						Stream: "Digest-pinned image appears to have been downloaded already\n",
					})
					mustPull = forcePull
				}
			}

			if mustPull {
				out := docker.NewOutStream(os.Stderr)
				_, isTerminal := term.GetFdInfo(os.Stderr)
				if err := dc.PullImage(
					egctx, image, platform, out,
					jsonmessage.WithMessagePrinter(batcher.PrintBatched(indent, isTerminal, 0)),
				); err != nil {
					return errors.Wrapf(err, "couldn't download %s", image)
				}
				batcher.Add(jsonstream.Message{Stream: "Finished download\n"})
			}

			if err := batcher.PrintRemaining(indent, 0); err != nil {
				return errors.Wrapf(err, "couldn't download %s", image)
			}

			p.Increment()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

type messageBatcher struct {
	muxer        *parallelPullMuxer
	progressLock *sync.Mutex
	headerMaker  func() string

	emitting bool
	messages []jsonstream.Message
	ids      map[string]uint
}

type parallelPullMuxer struct {
	lock       *sync.Mutex // mutex for the output streams and the header, to prevent interleaving
	lastHeader string
}

func newMessageBatcher(
	muxer *parallelPullMuxer, progressLock *sync.Mutex, headerMaker func() string,
) *messageBatcher {
	return &messageBatcher{
		muxer:        muxer,
		progressLock: progressLock,
		headerMaker:  headerMaker,
		messages:     make([]jsonstream.Message, 0),
		ids:          make(map[string]uint),
	}
}

func (b *messageBatcher) Add(jm jsonstream.Message) {
	b.messages = append(b.messages, jm)
}

func (b *messageBatcher) Clear() {
	b.messages = b.messages[:0]
}

func (b *messageBatcher) ReadyToEmitProgress() bool {
	if b.emitting {
		return true
	}
	if len(b.messages) < 1 {
		return false
	}

	b.emitting = b.progressLock.TryLock()
	return b.emitting
}

func (b *messageBatcher) PrintBatched(
	indent int, isTerminal bool, width uint16,
) func(jsonstream.Message, io.Writer) error {
	// This function is adapted from the displayJSONMessages function at
	// https://github.com/moby/moby/blob/master/client/pkg/jsonmessage/jsonmessage.go,
	// which is licensed under Apache-2.0. The function was changed to adapt the default
	// terminal-aware presentation for use with jsonmessage.WithMessagePrinter, and to replace
	// jsonmessage.Display with indentedDisplayMessage.
	return func(jm jsonstream.Message, out io.Writer) error {
		b.Add(jm)
		if !b.ReadyToEmitProgress() {
			return nil
		}

		if len(b.messages) == 0 {
			return nil
		}

		b.muxer.lock.Lock()
		defer func() {
			b.muxer.lock.Unlock()
			b.Clear()
		}()

		header := b.headerMaker()
		if header != b.muxer.lastHeader {
			IndentedFprintln(indent, os.Stderr, header)
			b.muxer.lastHeader = header
			// Clear the history of previous lines:
			b.ids = make(map[string]uint)
		}
		for _, jm := range b.messages {
			if err := b.printMessage(indent, jm, out, isTerminal, width); err != nil {
				return err
			}
		}
		return nil
	}
}

func (b *messageBatcher) printMessage(
	indent int, jm jsonstream.Message, out io.Writer, isTerminal bool, width uint16,
) error {
	var diff uint
	if jm.ID == "" || jm.Progress == nil {
		// When outputting something that isn't progress output, clear the history of previous lines.
		// We don't want progress entries from a batched message print for some other image to be
		// updated.
		b.ids = make(map[string]uint)
	} else {
		line, ok := b.ids[jm.ID]
		if !ok {
			// NOTE: This approach of using len(id) to figure out the number of lines of history only
			// works as long as we clear the history when we output something that's not accounted for
			// in the map, such as a line with no ID.
			line = uint(len(b.ids))
			b.ids[jm.ID] = line
			if isTerminal {
				_, _ = fmt.Fprintf(out, "\n")
			}
		}
		diff = uint(len(b.ids)) - line
		if isTerminal {
			cursorUp(out, diff)
		}
	}
	if err := indentedDisplayMessage(indent+1, jm, out, isTerminal, width); err != nil {
		return err
	}
	if jm.ID != "" && isTerminal {
		cursorDown(out, diff)
	}
	return nil
}

func (b *messageBatcher) PrintRemaining(indent int, width uint16) error {
	b.muxer.lock.Lock()
	defer func() {
		b.muxer.lock.Unlock()
		if b.emitting {
			b.progressLock.Unlock()
		}
	}()
	header := b.headerMaker()
	if header != b.muxer.lastHeader {
		IndentedFprintln(indent, os.Stderr, header)
		b.muxer.lastHeader = header
	}
	for _, jm := range b.messages {
		if err := indentedDisplayMessage(indent+1, jm, os.Stderr, false, width); err != nil {
			return err
		}
	}
	return nil
}

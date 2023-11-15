package logtransports

// A transport wrapper that moves the IO operation to a background thread by accumulating logged data and flushing it
// on a given time interval

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
)

// Queue size limits to for when the writer thread falls behind the event transformation.
// At these values:
//
//     Since we use a single mutex to guard insertion into the queue - the insertion point is effectively single threaded. This means the
//     max insertion speed is limited to how quickly a single thread can create LogRecords and append them to the queue. The transport threads
//     read the queue every 100 ms and will fall behind if the number of events inserted takes longer than 100 ms to process. The numbers below
//     control what happens if the transport thread falls behind and events are dropped to protect the process from running out of memory.
//     Below is the analysis for how much load each transport can handle:
//
//     LogServerTransport can process up to 5,000,0000 events a second for events without payload without dropping any of them. Because
//     events without payload are aggregated into batches, regardless of their total number they result in one call to logserver a second
//     which posts at most one counter for each event in the map for events without payload. Each event with unique payload results in a
//     separate entry, but since they are compact we can accumulate a really large batch without problems. If we start generating more of these
//     events we'll need to send them more than once a second, but for now there is no issue. There is no back off on posting message via
//     LogServerTransport so messages just accumulate until max of 200,000 and then get dropped, since we don't use this code path today
//     (it is meant for code running on customer's machines) - I left it as is.
//
//
//     FileTransport can process up to 1,5000,000 log lines a second without dropping any events. This translates to about 250MB a sec or
//     15 Gb every minute. At this rate we would fill up the disk in a few minutes. It will start dropping events if we log them faster,
//     than a single thread can write them to disk. If that ever becomes a problem we will need to make that transport multithreaded and
//     stripe the file.
//
//     GoTransport is synchronous and can process about 100,000 log lines a second before blocking the calling code. We should
//
//     Kinesis logger can process 3,500 log lines a second outside DC and 35,000 log lines a second for in region stream in DC. This is by far
//     the lowest limit and it is driven by the size of the batch 500 items and roundtrip cost to the kinesis server. Two mechanisms for increasing
//     the throughput rate are packing more log lines into each batch (will add shortly) and adding multiple threads to make multiple
//     post to kinesis at the same time (will add only if needed)

const (
	debugBackOffSize   = 200000
	infoBackOffSize    = 350000
	warningBackoffSize = 500000
	errorBackOffSize   = 550000
	maxQueueSize       = 600000
)

type logRecord struct {
	timestamp time.Time
	event     uclog.LogEvent
	next      *logRecord
}

// wrappedIOTransport defines the interface for wrapped transport that performs IO on background thread
type wrappedIOTransport interface {
	init(ctx context.Context) (*uclog.TransportConfig, error)
	writeMessages(ctx context.Context, logRecords *logRecord, startTime time.Time, count int)
	getIOInterval() time.Duration
	getMaxLogLevel() uclog.LogLevel
	getTransportName() string
	supportsCounters() bool
	flushIOResources()
	closeIOResources()
	getFailedAPICallsCount() int64
}

type transportBackgroundIOWrapper struct {
	w wrappedIOTransport

	writeMutex        sync.Mutex
	postMutext        sync.Mutex
	diskRecords       *logRecord
	writeTicker       time.Ticker
	queueSize         int64
	droppedEventCount int64
	sentEventCount    int64
	runningBGThread   bool
	done              chan bool
	exitBG            chan bool
	flushChan         chan bool
}

// newTransportBackgroundIOWrapper returns a wrapper around a uninitialized wrappedIOTransport
func newTransportBackgroundIOWrapper(w wrappedIOTransport) *transportBackgroundIOWrapper {
	var wrapper transportBackgroundIOWrapper
	wrapper.w = w
	return &wrapper
}

func (t *transportBackgroundIOWrapper) Init() (*uclog.TransportConfig, error) {
	ctx := context.Background() // TODO we may want to create unique context for background operations
	c, err := t.w.init(ctx)
	if err != nil {
		return c, ucerr.Wrap(err)
	}
	t.queueSize = 0
	// Launch the file writer thread to prevent excessive disk seeks when many threads log at once
	t.writeTicker = *time.NewTicker(t.w.getIOInterval())
	t.done = make(chan bool)
	t.exitBG = make(chan bool)
	t.flushChan = make(chan bool)
	go func() {
		t.runningBGThread = true
		for {
			select {
			case <-t.done:
				t.writeMutex.Lock()
				t.writeToIO(ctx)
				t.w.closeIOResources()
				t.runningBGThread = false
				t.writeMutex.Unlock()
				t.exitBG <- true
				return
			case <-t.flushChan:
				t.writeMutex.Lock()
				t.writeToIO(ctx)
				t.w.flushIOResources()
				t.writeMutex.Unlock()
			case <-t.writeTicker.C:
				t.writeMutex.Lock()
				t.writeToIO(ctx)
				t.writeMutex.Unlock()
			}
		}
	}()

	return c, nil
}

func (t *transportBackgroundIOWrapper) writeToIO(ctx context.Context) {
	var records *logRecord

	t.postMutext.Lock()
	records = t.diskRecords
	t.diskRecords = nil
	t.postMutext.Unlock()

	// Reverse the records since they are Last -> First order and count them
	var next *logRecord
	var recordCount = 0
	var startTime time.Time

	// Grab the earliest time in the batch
	if records != nil {
		startTime = records.timestamp
	}
	// Reverse the records
	for records != nil {
		tmp := records.next
		records.next = next
		next = records
		records = tmp
		recordCount++
	}
	records = next
	atomic.AddInt64(&t.queueSize, int64(-recordCount))
	atomic.AddInt64(&t.sentEventCount, int64(recordCount))

	t.w.writeMessages(ctx, records, startTime, recordCount)
}

func (t *transportBackgroundIOWrapper) queueCapacityBackoff() uclog.LogLevel {
	// Default case when the queue is not overloaded
	if t.queueSize < debugBackOffSize {
		return uclog.LogLevelVerbose
	}
	if t.queueSize < infoBackOffSize {
		return uclog.LogLevelDebug
	}
	if t.queueSize < warningBackoffSize {
		return uclog.LogLevelInfo
	}
	if t.queueSize < errorBackOffSize {
		return uclog.LogLevelWarning
	}
	if t.queueSize < maxQueueSize {
		return uclog.LogLevelError
	}
	// Drop the message
	return uclog.LogLevelNone
}

func (t *transportBackgroundIOWrapper) writeLogRecord(ctx context.Context, event *uclog.LogEvent) {
	// Check if the queue has space for this event to protect from OOM when the writer is too slow to keep up
	bL := t.queueCapacityBackoff()
	if bL < event.LogLevel || bL <= uclog.LogLevelWarning && event.LogLevel == uclog.LogLevelNonMessage {
		atomic.AddInt64(&t.droppedEventCount, 1)
		return
	}

	// Append the record to the front of the linked list
	t.postMutext.Lock()
	var record = logRecord{time.Now().UTC(), *event, t.diskRecords}
	t.diskRecords = &record
	t.queueSize++
	t.postMutext.Unlock()
}

func (t *transportBackgroundIOWrapper) WriteMessage(ctx context.Context, message string, level uclog.LogLevel) {
	t.writeLogRecord(ctx, &uclog.LogEvent{Message: message, Code: uclog.EventCodeNone, Count: 1, LogLevel: level})
}
func (t *transportBackgroundIOWrapper) WriteCounter(ctx context.Context, event uclog.LogEvent) {
	if t.w.supportsCounters() {
		t.writeLogRecord(ctx, &event)
	} else if event.Message != "" && event.LogLevel <= t.w.getMaxLogLevel() {
		t.WriteMessage(ctx, event.Message, event.LogLevel)
	}
}

func (t *transportBackgroundIOWrapper) GetStats() uclog.LogTransportStats {
	return uclog.LogTransportStats{
		Name:                t.w.getTransportName(),
		QueueSize:           t.queueSize,
		DroppedEventCount:   t.droppedEventCount,
		SentEventCount:      t.sentEventCount,
		FailedAPICallsCount: t.w.getFailedAPICallsCount(),
	}

}

func (t *transportBackgroundIOWrapper) GetName() string {
	if t.w != nil {
		return t.w.getTransportName()
	}
	return ""
}

func (t *transportBackgroundIOWrapper) Flush() error {
	if t.runningBGThread {
		t.flushChan <- true
	}
	return nil
}

func (t *transportBackgroundIOWrapper) Close() {
	if t.runningBGThread {
		t.writeTicker.Stop()
		// Send signal to background thread to perform final flush
		t.done <- true
		// Wait for the flush to finish
		<-t.exitBG
	}
}

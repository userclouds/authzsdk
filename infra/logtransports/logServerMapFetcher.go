package logtransports

// Transport directing event stream to our server
import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
	"userclouds.com/infra/uclog"
	logServerInterface "userclouds.com/logserver/client"
)

const (
	defaultEventMetadataDownloadInterval time.Duration = time.Second
	eventMetadataURL                     string        = "/eventmetadata/default"
)

type logServerMapFetcher struct {
	instanceID             uuid.UUID
	service                string
	logServiceURL          string
	eventMetadataRequests  []uuid.UUID
	queueLock              sync.Mutex
	updateEventDataHandler func(updatedMap *uclog.EventMetadataMap, tenantID uuid.UUID) error

	fetchMutex      sync.Mutex
	fetchTicker     time.Ticker
	done            chan bool
	runningBGThread bool

	failedServerCalls int
}

// getJSON is a simple helper that exists to avoid circular dependencies between
// infra/uclog, infra/jsonclient, and other libraries.
func getJSON(url string, response interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return ucerr.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusBadRequest {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return ucerr.Wrap(err)
		}
	} else {
		return ucerr.Errorf("error in GET to %s, code: %d", url, resp.StatusCode)
	}
	return nil
}

func newLogServerMapFetcher(logServiceURL string, service string) *logServerMapFetcher {
	var t = logServerMapFetcher{logServiceURL: logServiceURL, service: service}

	return &t
}

func (l *logServerMapFetcher) createURL(tenantID uuid.UUID) string {
	return l.logServiceURL + eventMetadataURL + "?" + logServerInterface.InstanceIDQueryArgName + "=" + l.instanceID.String() + "&" +
		logServerInterface.ServiceQueryArgName + "=" + l.service + "&" + logServerInterface.TenantIDQueryArgName + "=" +
		tenantID.String()
}

func (l *logServerMapFetcher) Init(updateHandler func(updatedMap *uclog.EventMetadataMap, tenantID uuid.UUID) error) error {
	l.instanceID = uuid.Must(uuid.NewV4())

	// Setup event metadata state
	l.eventMetadataRequests = make([]uuid.UUID, 0)
	l.updateEventDataHandler = updateHandler

	l.fetchTicker = *time.NewTicker(defaultEventMetadataDownloadInterval)
	l.done = make(chan bool)
	ctx := context.Background() // TODO we may want to create unique context for background operations
	go func() {
		l.runningBGThread = true
		for {
			select {
			case <-l.done:
				l.fetchMutex.Lock()
				l.runningBGThread = false
				l.fetchMutex.Unlock()
				return
			case <-l.fetchTicker.C:
				l.fetchMutex.Lock()
				l.updateEventMetadata(ctx)
				l.fetchMutex.Unlock()
			}
		}
	}()

	return nil
}

// FetchEventMetadataForTenant tries to fetch the event metadata map for given tenant
func (l *logServerMapFetcher) FetchEventMetadataForTenant(tenantID uuid.UUID) {
	l.queueLock.Lock()
	l.eventMetadataRequests = append(l.eventMetadataRequests, tenantID)
	l.queueLock.Unlock()
}

func (l *logServerMapFetcher) updateEventMetadata(ctx context.Context) {
	// Check if there are any requests and copy them into a local array
	l.queueLock.Lock()
	q := l.eventMetadataRequests
	l.eventMetadataRequests = make([]uuid.UUID, 0)
	l.queueLock.Unlock()

	for _, tenantID := range q {
		var newEventMetadata uclog.EventMetadataMap
		if err := getJSON(l.createURL(tenantID), &newEventMetadata); err == nil {
			if err := l.updateEventDataHandler(&newEventMetadata, tenantID); err == nil {
				continue
			}
		}
		l.FetchEventMetadataForTenant(tenantID)
		l.failedServerCalls++
	}
}

func (l *logServerMapFetcher) Close() {
	if l.runningBGThread {
		l.fetchTicker.Stop()
		// Send signal to background thread to perform final flush
		l.done <- true
	}
}

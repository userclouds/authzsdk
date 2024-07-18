package uclog // TODO move this to better place once it is more filled out

import (
	"fmt"
	"os"
	"time"
)

// LocalStatus contains basic approximate statistics about the service
type LocalStatus struct {
	CallCount          int       `json:"callcount"`           // total calls received by the service
	InputErrorCount    int       `json:"input_errorcount"`    // number of input errors
	InternalErrorCount int       `json:"internal_errorcount"` // number of internal errors
	LastCall           time.Time `json:"lastcall_time"`       // timestamp of last successful call
	LastErrorCall      time.Time `json:"lasterror_time"`      // timestamp of last error
	LastErrorCode      int       `json:"lasterror_code"`      // last error code
	ComputeTime        int       `json:"computetime"`         // amount of time spent in handlers

	LoggerStats []LogTransportStats `json:"loggerstats"`
}

var status LocalStatus

// GetStatus return approximate statistics about the service
func GetStatus() LocalStatus {
	status.LoggerStats = GetStats()
	return status
}

// Hostname centralizes our code to figure out what machine we're on
// TODO: this isn't the right place for this to live, but the wrong code already got
// copy-pasted across logging so at least this will start the fix. It could be in the service
// package but that ends up importing migrate -> uclog and I don't want to untangle that
// right now. GetStatus shouldn't live in uclog either but add it to the list. :)
func Hostname() string {
	// in AWS this will return something like `ip-10-1-0-108.us-west-2.compute.internal`, which is
	// mappable to an instance ID in the EC2 UI or CLI
	// if it turns out to be more helpful to get the instance ID, you can use
	// `exec.Command("ec2-metadata", "-i").Output()` and `strings.TrimPrefix(output, "instance-id: ")` to get this
	// but at that point you have to handle dev environments differently since `ec2-metadata` obviously doesn't exist
	host, err := os.Hostname()
	if err != nil {
		host = fmt.Sprintf("error getting hostname: %v", err)
	}
	return host
}

// updateStatus updates stats, last writer wins some the results are approximate
func (s *LocalStatus) updateStatus(e LogEvent, t LogEventTypeInfo) {
	if e.Code == EventCodeNone {
		return
	}

	if t.Category == "Call" {
		s.CallCount++
		s.LastCall = time.Now().UTC()
		return
	}

	if t.Category == "Duration" {
		s.ComputeTime = s.ComputeTime + e.Count
		return
	}

	if t.Category == "InternalError" || t.Category == "InputError" {
		if t.Category == "InternalError" {
			s.InternalErrorCount++
		} else {
			s.InputErrorCount++
		}
		s.LastErrorCode = int(e.Code)
		s.LastErrorCall = time.Now().UTC()
	}
}

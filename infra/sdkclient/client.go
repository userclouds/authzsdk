package sdkclient

import (
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/uclog"
)

// Client represents a jsonclient that communicates with the UserClouds API
type Client struct {
	*jsonclient.Client
}

// New constructs a new UserClouds SDK client
func New(url string, opts ...jsonclient.Option) *Client {
	opts = append(opts, jsonclient.Header(uclog.HeaderSDKVersion, sdkVersion))
	c := jsonclient.New(url, opts...)

	return &Client{c}
}

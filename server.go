package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// App represents the server's internal state.
// It holds configuration about providers and content
type App struct {
	ContentClients map[Provider]Client
	Config         ContentMix
}

// ContentResponse represents type used to wrap a provider's content result and error into a single struct
type ContentResponse struct {
	Content []*ContentItem
	Error   error
}

// getQueryParameters gets count and offset query parameters
func getQueryParameters(r *http.Request) (count, offset int) {
	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count < 0 {
		count = 0
	}
	offset, err = strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	return
}

// channelToResponseWriter : marshals slice of ContentItem structs and writes them to http writer
func channelToResponseWriter(val []*ContentItem, w *http.ResponseWriter) {
	marsh, _ := json.Marshal(val[0])
	(*w).Write(marsh)
}

// ServeHTTP : main HTTP Handler for GET requests on server
// Concurrently executes getting content from providers, in manner detailed in README.md
func (a App) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Status", "200")
	w.Header().Set("Content-Type", "application/json")
	log.Printf("%s %s", req.Method, req.URL.String())
	count, offset := getQueryParameters(req)

	// write first characters of response, which is going to be an array
	w.Write([]byte("["))
	defer w.Write([]byte("]"))

	// slice of channels, one for each content provider in configuration
	// using this, we can concurrently get content from each provider and get the data back in a specific order
	chans := make([]chan ContentResponse, len(a.Config))
	for i := 0; i < len(a.Config); i++ {
		chans[i] = make(chan ContentResponse)
	}

	// holds total number of content bits taken from providers
	totalCount := 0
	// boolean parameter that is used to stop content fetching if number of desired entries has already been fetched
	done := false
	i := 0
	// boolean parameter that determines if the current loop corresponds to the first batch of content
	// is mostly used to generate the JSON byte string correctly
	onFirstWrite := true

	// Loop which concurrently executes one content request per provider in batches, until desired articles count is met
ContentBatchLoop:
	for {
		// start of a new batch of content

		// check if we have already fetched the desired number of articles
		if done {
			break ContentBatchLoop
		}

		// === Channel SEND operations section ===
		// starting at the original offset position, go through providers until the last one in the config
		// each provider has it's own assigned channel which it will send the content to
		for i = offset; i < len(a.Config); i++ {
			// check that we haven't already fetched enough data from providers
			if totalCount == count {
				done = true
				break
			}
			totalCount++

			// concurrently get content from provider
			// note that each provider of the app has it's own assigned channel (chans[i])
			go a.getContent(i, chans[i])
		}

		// edge case. TODO: find way to not need this anymore
		if i == 0 {
			break
		}

		// === Channel RECEIVE operations section ===
		// note that we are using unbuffered channels, so each SEND operation must have a corresponding RECEIVE operation
		// j < i syntax makes sure that we do not do more RECEIVE operations that SEND ones executed earlier
		// A case where that might happen is when the break is called. In that case, i won't go all the way to len(a.Config)
		for j := offset; j < i; j++ {
			// get value from provider's channel
			val := <-chans[j]
			if val.Error != nil {
				// check if there is an error attached to the data on the channel, stop operations
				break ContentBatchLoop
			}

			// we check if we are on the first batch of data, case when we don't want to add "," initially
			// for all other cases, we can add ","
			if !onFirstWrite {
				w.Write([]byte(","))
			}
			onFirstWrite = false

			// if value does not have an error attached, we can write it to the http writer
			channelToResponseWriter(val.Content, &w)

		}

		// after the first pass, we can loop another batch of provider requests starting with the first one
		offset = 0
	}

	// close all channels for each config provider
	// at this point, if there was an error and the Mainloop was stopped forcefully, we may still have data on some of these
	// Although, that data does not end up in the response and is discarded by garbage collection
	for i := 0; i < len(a.Config); i++ {
		close(chans[i])
	}

	fmt.Println("\nDone")
	return
}

// getContent gets content from one of the application's providers
func (a App) getContent(idx int, ch chan ContentResponse) {
	// first attempt to call main content client
	// it is accessed by going through the providers-clients map
	// and finding the corresponding client for a specific provider, which gives access to it's GetContent method
	content, err := a.ContentClients[a.Config[idx].Type].GetContent("Test", 1)
	if err != nil {
		fallbackContent, fallbackErr := a.ContentClients[*a.Config[idx].Fallback].GetContent("Test", 1)
		// send content and potential error to channel
		ch <- ContentResponse{
			Content: fallbackContent,
			Error:   fallbackErr,
		}
		return
	}
	// send content and potential error to channel
	ch <- ContentResponse{
		Content: content,
		Error:   nil,
	}
	return
}

package main

import (
	"errors"
	"fmt"
	"github.com/go-zookeeper/zk"
	"github.com/youngkin/GoZKLeaderElection/leader"
	"strings"
	"sync"
	"time"
)

//Tests:
//	1. Of multiple candidates, only 1 becomes leader
//	2. When a leader of multiple candidates resigns, one of the remaining candidates is chosen as leader
//	3. When the last leader resigns there is no leader and no remaining candidates
//	4. When the ZK connection is lost all candidates are notified and WHAT HAPPENS???
//	5. How does it work in a distributed (i.e., multi-process/multi-host) environment
//		i. When ZK connection is lost to one ZK in a cluster
//			a. All happy-path scenarios still work
//

type ElectionResponse struct {
	IsLeader    bool
	CandidateID string
}

func main() {
	respCh := make(chan ElectionResponse)
	conn, connEvtChan := connect("192.168.12.11:2181")

	// Monitor channel events
	go func() {
		var retries int
		for {
			evt := <-connEvtChan
			fmt.Println("Received channel event:", evt)
			if evt.State == zk.StateDisconnected {
				if retries > 5 {
					// This is what essentially what Election clients should do
					panic(errors.New("OMG, lost connectivity to ZK"))
				}
				retries++
			}
		}
	}()

	// Wait to log connect events from above goroutine
	time.Sleep(2 * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go runCandidate(conn, "/election", &wg, respCh, uint(i))
	}

	go func() {
		wg.Wait()
		close(respCh)
	}()

	responses := make([]ElectionResponse, 0)
	var i int
	for response := range respCh {
		fmt.Println("Election result", i, ":", response)
		i++
		responses = append(responses, response)
	}

	//	fmt.Println("\n\nCandidates at end:", le.String())
	verifyResults(responses)
}

func connect(zksStr string) (*zk.Conn, <-chan zk.Event) {
	//	zksStr := os.Getenv("ZOOKEEPER_SERVERS")
	zks := strings.Split(zksStr, ",")
	conn, connEvtChan, err := zk.Connect(zks, 5*time.Second)
	must(err)
	return conn, connEvtChan
}

func must(err error) {
	if err != nil {
		fmt.Println("must(", err, ") called")
		panic(err)
	}
}

func runCandidate(zkConn *zk.Conn, electionPath string, wg *sync.WaitGroup, respCh chan ElectionResponse, waitFor uint) {
	leaderElector, err := leader.NewElection(zkConn, "/election")
	must(err)
	//	fmt.Println(leaderElector.String(), "\n\n")

	isLeader, candidate, ldrshpChgChnl := leaderElector.ElectLeader("n_", "president")
	//	fmt.Println("leaderElector AFTER ELECTION: leaderElector.IsLeader(", id, ")?:", leaderElector.IsLeader(id))
	if isLeader {
		respCh <- ElectionResponse{isLeader, candidate.CandidateID}
	}

	for !isLeader {
		select {
		case isLeader = <-ldrshpChgChnl:
			fmt.Println("\t\tGot leadership change event for", candidate.CandidateID, ", am I leader?", isLeader)
			if isLeader {
				respCh <- ElectionResponse{isLeader, candidate.CandidateID}
			}
		case <-time.NewTimer(100 * time.Second).C:
			fmt.Println("Timer expired, stop waiting to become leader for", candidate.CandidateID)
			wg.Done()
			return
		}
	}

	// do some work when I become leader
	sleepSeconds := waitFor*waitFor + 1
	time.Sleep(time.Duration(sleepSeconds) * time.Second)

	err = leaderElector.Resign(candidate)
	if err != nil {
		fmt.Println("leaderElector AFTER FAILED RESIGN: error is", err)
	}

	wg.Done()
}

func verifyResults(responses []ElectionResponse) {
	testPassed := true
	numResponses := 0
	for _, leaderResp := range responses {
		if leaderResp.IsLeader != true {
			fmt.Println("Test failed!!!! for candidate:", leaderResp)
			testPassed = false
			break
		}
		numResponses++
	}
	if numResponses != 3 {
		fmt.Println("Test failed!!! Expected 3 responses but received", numResponses)
		testPassed = false
	}
	if testPassed {
		fmt.Println("\nTEST PASSED, HOORAY!!!")
	} else {
		fmt.Println("\nTEST FAILED, unexpected failed election or wrong number of leaders elected")
	}
}

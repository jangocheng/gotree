// Copyright gotree Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License")
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	sysStartTime = time.Now()
	x := int64(rand.Intn(10000))
	identification = strconv.FormatInt(x, 36)
	tseq = 1
}

var internalLock sync.Mutex

// memory.
var memStats runtime.MemStats

// internal
type internalb struct {
	Name string
	F    func(this *gTimer)
}

type gTimer struct {
	mu      sync.RWMutex
	output  []byte
	w       io.Writer
	ran     bool
	failed  bool
	skipped bool
	done    bool
	helpers map[string]struct{}

	chatty           bool
	finished         bool
	hasSub           int32
	raceErrors       int
	runner           string
	level            int
	creator          []uintptr
	name             string
	start            time.Time
	duration         time.Duration
	barrier          chan bool
	signal           chan bool
	importPath       string
	context          *benchContext
	N                int
	previousN        int
	previousDuration time.Duration
	benchFunc        func(this *gTimer)
	benchTime        time.Duration
	bytes            int64
	missingBytes     bool
	timerOn          bool
	showAllocResult  bool
	result           timerResult
	parallelism      int
	startAllocs      uint64
	startBytes       uint64
	netAllocs        uint64
	netBytes         uint64
}

// StartTimer
func (this *gTimer) StartTimer() {
	if !this.timerOn {
		runtime.ReadMemStats(&memStats)
		this.startAllocs = memStats.Mallocs
		this.startBytes = memStats.TotalAlloc
		this.start = time.Now()
		this.timerOn = true
	}
}

// StopTimer
func (this *gTimer) StopTimer() {
	if this.timerOn {
		this.duration += time.Since(this.start)
		runtime.ReadMemStats(&memStats)
		this.netAllocs += memStats.Mallocs - this.startAllocs
		this.netBytes += memStats.TotalAlloc - this.startBytes
		this.timerOn = false
	}
}

// ResetTimer
func (this *gTimer) ResetTimer() {
	if this.timerOn {
		runtime.ReadMemStats(&memStats)
		this.startAllocs = memStats.Mallocs
		this.startBytes = memStats.TotalAlloc
		this.start = time.Now()
	}
	this.duration = 0
	this.netAllocs = 0
	this.netBytes = 0
}

// SetBytes
func (this *gTimer) SetBytes(n int64) { this.bytes = n }

// ReportAllocs
func (this *gTimer) ReportAllocs() {
	this.showAllocResult = true
}

func (this *gTimer) nsPerOp() int64 {
	if this.N <= 0 {
		return 0
	}
	return this.duration.Nanoseconds() / int64(this.N)
}

// runN
func (this *gTimer) runN(n int) {
	internalLock.Lock()
	defer internalLock.Unlock()
	// Try
	runtime.GC()
	this.N = n
	this.parallelism = 1
	this.ResetTimer()
	this.StartTimer()
	this.benchFunc(this)
	this.StopTimer()
	this.previousN = n
	this.previousDuration = this.duration
}

func min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// roundDown10
func roundDown10(n int) int {
	var tens = 0

	for n >= 10 {
		n = n / 10
		tens++
	}

	result := 1
	for i := 0; i < tens; i++ {
		result *= 10
	}
	return result
}

// roundUp
func roundUp(n int) int {
	base := roundDown10(n)
	switch {
	case n <= base:
		return base
	case n <= (2 * base):
		return 2 * base
	case n <= (3 * base):
		return 3 * base
	case n <= (5 * base):
		return 5 * base
	default:
		return 10 * base
	}
}

// run1
func (this *gTimer) run1() bool {
	if ctx := this.context; ctx != nil {
		// Extend
		if n := len(this.name) + ctx.extLen + 1; n > ctx.maxLen {
			ctx.maxLen = n + 8
		}
	}
	go func() {
		// Signal
		defer func() {
			this.signal <- true
		}()

		this.runN(1)
	}()
	<-this.signal
	if this.failed {
		fmt.Fprintf(this.w, "--- FAIL: %s\n%s", this.name, this.output)
		return false
	}
	// Only p
	if atomic.LoadInt32(&this.hasSub) != 0 || this.finished {
		tag := "BENCH"
		if this.skipped {
			tag = "SKIP"
		}
		if this.chatty && (len(this.output) > 0 || this.finished) {
			this.trimOutput()
			fmt.Fprintf(this.w, "--- %s: %s\n%s", tag, this.name, this.output)
		}
		return false
	}
	return true
}

var labelsOnce sync.Once

// run
func (this *gTimer) run() {
	labelsOnce.Do(func() {
		fmt.Fprintf(this.w, "goos: %s\n", runtime.GOOS)
		fmt.Fprintf(this.w, "goarch: %s\n", runtime.GOARCH)
		if this.importPath != "" {
			fmt.Fprintf(this.w, "pkg: %s\n", this.importPath)
		}
	})
	this.doBench()
}

func (this *gTimer) doBench() timerResult {
	go this.launch()
	<-this.signal
	return this.result
}

// launch
func (this *gTimer) launch() {
	// Signal
	defer func() {
		this.signal <- true
	}()

	// Run
	d := this.benchTime
	for n := 1; !this.failed && this.duration < d && n < 1e9; {
		last := n
		// Predict r
		n = int(d.Nanoseconds())
		if nsop := this.nsPerOp(); nsop != 0 {
			n /= int(nsop)
		}
		// Run more iterations
		n = max(min(n+n/5, 100*last), last+1)
		// Round up
		n = roundUp(n)
		this.runN(n)
	}
	this.result = timerResult{this.N, this.duration, this.bytes, this.netAllocs, this.netBytes}
}

//  GetSysClock
func GetSysClock() int64 {
	return time.Now().Unix() - sysStartTime.Unix()
}

// The results of a run.
type timerResult struct {
	N         int
	T         time.Duration
	Bytes     int64
	MemAllocs uint64
	MemBytes  uint64
}

func (r timerResult) NsPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return r.T.Nanoseconds() / int64(r.N)
}

func (r timerResult) mbPerSec() float64 {
	if r.Bytes <= 0 || r.T <= 0 || r.N <= 0 {
		return 0
	}
	return (float64(r.Bytes) * float64(r.N) / 1e6) / r.T.Seconds()
}

// AllocsPerOp
func (r timerResult) AllocsPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemAllocs) / int64(r.N)
}

// AllocedBytesPerOp
func (r timerResult) AllocedBytesPerOp() int64 {
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemBytes) / int64(r.N)
}

func (r timerResult) String() string {
	mbs := r.mbPerSec()
	mb := ""
	if mbs != 0 {
		mb = fmt.Sprintf("\t%7.2f MB/s", mbs)
	}
	nsop := r.NsPerOp()
	ns := fmt.Sprintf("%10d ns/op", nsop)
	if r.N > 0 && nsop < 100 {
		// the ones
		if nsop < 10 {
			ns = fmt.Sprintf("%13.2f ns/op", float64(r.T.Nanoseconds())/float64(r.N))
		} else {
			ns = fmt.Sprintf("%12.1f ns/op", float64(r.T.Nanoseconds())/float64(r.N))
		}
	}
	return fmt.Sprintf("%8d\t%s%s", r.N, ns, mb)
}

// MemString
func (r timerResult) MemString() string {
	return fmt.Sprintf("%8d B/op\t%8d allocs/op",
		r.AllocedBytesPerOp(), r.AllocsPerOp())
}

// benchmarkName
func benchmarkName(name string, n int) string {
	if n != 1 {
		return fmt.Sprintf("%s-%d", name, n)
	}
	return name
}

var tseq int64
var identification string
var tseqMutex sync.Mutex

func getTseq() (result string) {
	defer tseqMutex.Unlock()
	tseqMutex.Lock()
	result = identification
	result += strconv.FormatInt(tseq, 36)
	if tseq == math.MaxInt64 {
		tseq = 1
		return
	}
	tseq += 1
	return
}

type benchContext struct {
	maxLen int
	extLen int
}

// add
func (this *gTimer) add(other timerResult) {
	r := &this.result
	// The aggregated
	// in sequence
	r.N = 1
	r.T += time.Duration(other.NsPerOp())
	if other.Bytes == 0 {
		// Summing Bytes
		this.missingBytes = true
		r.Bytes = 0
	}
	if !this.missingBytes {
		r.Bytes += other.Bytes
	}
	r.MemAllocs += uint64(other.AllocsPerOp())
	r.MemBytes += uint64(other.AllocedBytesPerOp())
}

// trimOutput
func (this *gTimer) trimOutput() {
	// The output
	const maxNewlines = 10
	for nlCount, j := 0, 0; j < len(this.output); j++ {
		if this.output[j] == '\n' {
			nlCount++
			if nlCount >= maxNewlines {
				this.output = append(this.output[:j], "\n\t... [output truncated]\n"...)
				break
			}
		}
	}
}

// A PB
type PB struct {
	globalN *uint64
	grain   uint64
	cache   uint64
	bN      uint64
}

// Next 下一个迭代
func (this *PB) Next() bool {
	if this.cache == 0 {
		n := atomic.AddUint64(this.globalN, this.grain)
		if n <= this.bN {
			this.cache = this.grain
		} else if n < this.bN+this.grain {
			this.cache = this.bN + this.grain - n
		} else {
			return false
		}
	}
	this.cache--
	return true
}

// RunEffect
func (this *gTimer) RunEffect(body func(*PB)) {
	if this.N == 0 {
		return
	}
	// Calculate
	grain := uint64(0)
	if this.previousN > 0 && this.previousDuration > 0 {
		grain = 1e5 * uint64(this.previousN) / uint64(this.previousDuration)
	}
	if grain < 1 {
		grain = 1
	}
	// We expect
	// so do
	if grain > 1e4 {
		grain = 1e4
	}

	n := uint64(0)
	numProcs := this.parallelism * runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(numProcs)
	for p := 0; p < numProcs; p++ {
		go func() {
			defer wg.Done()
			pb := &PB{
				globalN: &n,
				grain:   grain,
				bN:      uint64(this.N),
			}
			body(pb)
		}()
	}
	wg.Wait()
}

// SetParallelism
func (this *gTimer) SetParallelism(p int) {
	if p >= 1 {
		this.parallelism = p
	}
}

type discard struct{}

func (discard) Write(b []byte) (n int, err error) { return len(b), nil }

//  FileExists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

var sysStartTime time.Time
var currentTimeNum int32 = 0

func CurrentTimeNum() int {
	var result int32
	result = atomic.LoadInt32(&currentTimeNum)
	return int(result)
}

func RunTickStopTimer(tick int64, child TickStopRun) {
	t := timer{
		tickStopfunc: child,
		n:            tick,
	}
	go t.tickLoop()
}

type StopTick interface {
	Stop()
}

func RunTick(tick int64, child func(), name string, delay int) StopTick {
	t := timer{
		tickfunc: child,
		n:        tick,
	}
	t.stop = make(chan bool, 1)
	if delay > 0 {
		go func() {
			time.Sleep(time.Duration(delay) * time.Millisecond)
			t.triggerLoop(name)
		}()
	} else {
		go t.triggerLoop(name)
	}
	return &t
}

func RunDefaultTimer(tick int64, child func()) {
	t := timer{
		timefunc: child,
		n:        tick,
	}
	go t.defaultLoop()
}

func RunDay(hour, minute int, child func(), name string) {
	t := timer{
		timeDay: child,
		n:       0,
	}
	go t.dayLoop(hour, minute, name)
}

type TickStopRun func(stop *bool)

type timer struct {
	timefunc     func()
	tickStopfunc TickStopRun
	tickfunc     func()
	timeDay      func()
	n            int64
	stop         chan bool
}

func (this *timer) Stop() {
	this.stop <- true
}

func (this *timer) triggerLoop(name string) {
	atomic.AddInt32(&currentTimeNum, 1)
	defer func() {
		atomic.AddInt32(&currentTimeNum, -1)
		if perr := recover(); perr != nil {
			Log().Error(perr)
		}

		_runtimeStack.Remove()
	}()

	n := this.n
	if n > 1000 {
		n = 1000
	}
	var deltaTime int64 = int64(0)

	for {
		isBreak := false
		select {
		case stop := <-this.stop:
			isBreak = stop
		default:
			isBreak = false
		}
		if isBreak {
			break
		}

		if deltaTime >= this.n {
			_runtimeStack.Set("gseq", "t-"+getTseq())
			this.tickfunc()
			deltaTime = 0
		}

		time.Sleep(time.Duration(n) * time.Millisecond)
		deltaTime += n
	}
}

func (this *timer) tickLoop() {
	defer func() {
		if perr := recover(); perr != nil {
			Log().Error(perr)
		}
	}()
	for {
		stop := false
		this.tickStopfunc(&stop)
		if stop {
			break
		}
		time.Sleep(time.Duration(this.n) * time.Millisecond)
	}
}

func (this *timer) defaultLoop() {
	defer func() {
		if perr := recover(); perr != nil {
			Log().Error(perr)
		}
	}()
	for {
		time.Sleep(time.Duration(this.n) * time.Millisecond)
		this.timefunc()
	}
}

func (this *timer) dayLoop(hour, minute int, name string) {
	defer func() {
		if perr := recover(); perr != nil {
			Log().Error(perr)
		}
		_runtimeStack.Remove()
	}()

	currentHour := time.Now().Hour()
	currentMinute := time.Now().Minute()
	var n int64
	if currentHour > hour || (currentHour == hour && currentMinute >= minute) {
		td := time.Now().AddDate(0, 0, 1)
		local, _ := time.LoadLocation("Local")
		nt, _ := time.ParseInLocation("2006-01-02", td.Format("2006-01-02"), local)
		n = nt.Unix() + int64(hour*60*60+(minute*60))
	} else {
		td := time.Now()
		local, _ := time.LoadLocation("Local")
		nt, _ := time.ParseInLocation("2006-01-02", td.Format("2006-01-02"), local)
		n = nt.Unix() + int64(hour*60*60+(minute*60))
	}

	for {
		unix := time.Now().Unix()
		diff := n - unix
		diffTick := int(float32(diff) * float32(0.01))
		if unix > n {
			_runtimeStack.Set("gseq", "t-"+getTseq())
			this.timeDay()
			break
		}
		time.Sleep(time.Duration(diffTick) * time.Second)
	}

	time.Sleep(5 * time.Second)
	this.dayLoop(hour, minute, name)
}

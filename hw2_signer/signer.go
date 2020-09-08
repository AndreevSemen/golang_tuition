package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

func ExecutePipeline(jobs ...job) {
	// Temporary shared channel for keeping previous out channel
	// to be used as next in channel
	// ch0 -> job0 -> ch1 -> job1 -> ch2
	var sharedChan chan interface{}

	wg := &sync.WaitGroup{}
	for i := range jobs {
		out := make(chan interface{})

		// Creating workers' group
		// After end of job out channel must be closed
		// to notify next pipeline node about end of data stream
		wg.Add(1)
		go func(job job, in, out chan interface{}) {
			job(in, out)
			close(out)
			wg.Done()
		}(jobs[i], sharedChan, out)

		sharedChan = out
	}
	wg.Wait()
}

// Saves from overheat (data race) by using mutex which controls
// calling DataSignerMd5() by workers
func SaveMd5(data string, mut *sync.Mutex) string {
	mut.Lock()
	defer mut.Unlock()

	result := DataSignerMd5(data)
	return result
}

func SingleHash(in, out chan interface{}) {
	// Mutex for exclusion a data race
	md5Mut := &sync.Mutex{}

	wg := &sync.WaitGroup{}
	for val := range in {
		// Data preparation
		num, ok := val.(int)
		if !ok {
			panic("value is not int")
		}
		data := strconv.Itoa(num)

		// Run workers which calculates and return string:
		// crc32(data) + "~" + crc32(md5(data))
		wg.Add(1)
		go func() {
			var result1, result2 string

			// Creating 2 waited goroutines for calculating
			// parts of result
			crcWg := &sync.WaitGroup{}
			crcWg.Add(2)
			go func() {
				result1 = DataSignerCrc32(data)
				crcWg.Done()
			}()
			go func() {
				result2 = DataSignerCrc32(SaveMd5(data, md5Mut))
				crcWg.Done()
			}()
			crcWg.Wait()

			out <- result1 + "~" + result2

			wg.Done()
		}()
	}

	wg.Wait()
}

func MultiHash(in, out chan interface{}) {
	const CycleNum = 6

	wg := &sync.WaitGroup{}
	for val := range in {
		// Data preparation
		data, ok := val.(string)
		if !ok {
			panic("value is not string")
		}

		// Run workers which calculates and returns
		// result string in parallel:
		/* Result string have following structure
			var result string
			for th := 0; th < 6; th++ {
				result += crc(th + data)
			}
		}*/
		wg.Add(1)
		go func() {
			var results [CycleNum]string

			cycleWg := &sync.WaitGroup{}
			for th := 0; th < CycleNum; th++ {
				cycleWg.Add(1)
				go func(th int) {
					results[th] = DataSignerCrc32(strconv.Itoa(th) + data)
					cycleWg.Done()
				}(th)
			}
			cycleWg.Wait()

			out <- strings.Join(results[:], "")

			wg.Done()
		}()
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var vals []string
	for val := range in {
		vals = append(vals, val.(string))
	}

	sort.Strings(vals)
	out <- strings.Join(vals, "_")
}

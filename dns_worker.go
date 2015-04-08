package main

func worker() {
	for job := range queryChan {
		job.run()
	}
}

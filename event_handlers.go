package serf

func (s *Serf) receiveEvents() {
	for {
		select {
		case e := <-s.outEventCh:
			s.logger.Printf("[INFO] serf: received event: %s", e.String())
			s.invokeEventScript("echo Hello", e)
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting event reception")
			return
		}
	}
}

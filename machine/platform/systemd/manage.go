package systemd

import "github.com/kardianos/service"

type startStop struct{}

func (p *startStop) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *startStop) run() {
	// Do work here
}
func (p *startStop) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

package main

type RedisMock struct {
	CloseFunc   func() error
	ErrFunc     func() error
	DoFunc      func(command string, args ...interface{}) (interface{}, error)
	SendFunc    func(command string, args ...interface{}) error
	FlushFunc   func() error
	ReceiveFunc func() (interface{}, error)
}

func (rm *RedisMock) Close() error {
	if rm.CloseFunc == nil {
		return nil
	}
	return rm.CloseFunc()
}

func (rm *RedisMock) Err() error {
	if rm.ErrFunc == nil {
		return nil
	}
	return rm.ErrFunc()
}

func (rm *RedisMock) Do(command string, args ...interface{}) (interface{}, error) {
	if rm.DoFunc == nil {
		return nil, nil
	}
	return rm.DoFunc(command, args...)
}

func (rm *RedisMock) Send(command string, args ...interface{}) error {
	if rm.SendFunc == nil {
		return nil
	}
	return rm.SendFunc(command, args...)
}

func (rm *RedisMock) Flush() error {
	if rm.FlushFunc != nil {
		return nil
	}
	return rm.FlushFunc()
}

func (rm *RedisMock) Receive() (interface{}, error) {
	if rm.ReceiveFunc == nil {
		return nil, nil
	}
	return rm.ReceiveFunc()
}

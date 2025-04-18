package common

type Callback func()

var FinishedCallbacks []Callback

func AddCallback(cb Callback) {
	FinishedCallbacks = append(FinishedCallbacks, cb)
}

var OutputDir = GetOutputDir()

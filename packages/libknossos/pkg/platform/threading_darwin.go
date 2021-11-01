package platform

// extern void run_on_main(int id);
import "C"
import "sync"

var (
	callbackMap    map[int]func() = map[int]func(){}
	callbackNextID int            = 0
	callbackMutex  sync.RWMutex
)

//export libknossos_run_item
func libknossos_run_item(id C.int) {
	callbackMutex.Lock()
	callback := callbackMap[int(id)]
	delete(callbackMap, int(id))
	callbackMutex.Unlock()

	callback()
}

func RunOnMain(callback func()) {
	callbackMutex.Lock()
	id := callbackNextID
	callbackNextID++
	callbackMap[id] = callback
	callbackMutex.Unlock()

	C.run_on_main(C.int(id))
}

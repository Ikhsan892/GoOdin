package listener

type OrderDetailListener interface {
	Start() error
	Stop() error
}

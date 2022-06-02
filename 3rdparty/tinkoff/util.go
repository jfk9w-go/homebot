package tinkoff

const Host = "https://www.tinkoff.ru"

type operationSort []Operation

func (os operationSort) Len() int {
	return len(os)
}

func (os operationSort) Less(i, j int) bool {
	return os[i].ID < os[j].ID
}

func (os operationSort) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}

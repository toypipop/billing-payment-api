package dto

// InvoicePage bounds database reads, including calls outside HTTP.
type InvoicePage struct {
	Limit   int
	AfterID uint
}

func (p InvoicePage) Normalize() InvoicePage {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	return p
}

package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	unitNumber   string
	streetNumber string
	streetName   string
	suburb       string
	postcode     string
	voucherCode  string
)

func init() {
	flag.StringVar(&unitNumber, "unit_number", "", "Unit Number")
	flag.StringVar(&streetNumber, "street_number", "", "Street Number")
	flag.StringVar(&streetName, "street_name", "", "Street Name")
	flag.StringVar(&suburb, "suburb", "", "Suburb")
	flag.StringVar(&postcode, "postcode", "", "Postcode")
	flag.StringVar(&voucherCode, "voucher_code", "", "Voucher Code")
}

func main() {
	flag.Parse()

	c := NewClient()

	fd, err := os.OpenFile("results.txt", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	if err := c.Login(Address{
		UnitNumber:   unitNumber,
		StreetNumber: streetNumber,
		StreetName:   streetName,
		Suburb:       suburb,
		Postcode:     postcode,
	}); err != nil {
		panic(err)
	}

	fmt.Printf("[ ] Trying code %s\n", voucherCode)

	if err := c.ApplyVoucher(voucherCode); err != nil {
		fmt.Printf("[!] Error applying code %s: %s\n", voucherCode, err.Error())
		os.Exit(1)
	}

	b, err := c.GetBasket()
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(fd, "Code: %s\n", voucherCode)

	for _, e := range b.Vouchers {
		fmt.Printf("[+] Voucher %s: %s %s (%s)\n", e.ItemID, e.Code, e.Price, e.Name)
		fmt.Fprintf(fd, "  Voucher %s %s (%s)\n", e.Code, e.Price, e.Name)
	}
	for _, e := range b.Items {
		fmt.Printf("[+] Item %s: %s (%s)\n", e.ItemID, e.ProductCode, e.Name)
		fmt.Fprintf(fd, "  Item %s (%s)\n", e.ProductCode, e.Name)
	}

	for len(b.Items) > 0 {
		item := b.Items[0]

		fmt.Printf("[!] Removing item %s (%s)\n", item.ItemID, item.Name)

		bb, err := c.RemoveItem(item.ItemID)
		if err != nil {
			panic(err)
		}

		b = bb
	}

	for len(b.Vouchers) > 0 {
		voucher := b.Vouchers[0]

		fmt.Printf("[!] Removing voucher %s (%s)\n", voucher.ItemID, voucher.Name)

		if err := c.RemoveVoucher(voucher.ItemID); err != nil {
			panic(err)
		}

		bb, err := c.GetBasket()
		if err != nil {
			panic(err)
		}

		b = bb
	}

	for _, e := range b.Vouchers {
		fmt.Printf("Voucher: %s %s (%s)\n", e.Code, e.Price, e.Name)
	}
	for _, e := range b.Items {
		fmt.Printf("Item: %s (%s)\n", e.ProductCode, e.Name)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Client struct {
	hc *http.Client
}

func NewClient() *Client {
	j, _ := cookiejar.New(nil)

	return &Client{
		hc: &http.Client{Transport: http.DefaultTransport, Jar: j},
	}
}

type Address struct {
	UnitNumber   string
	StreetNumber string
	StreetName   string
	Suburb       string
	Postcode     string
}

func (c *Client) Login(a Address) error {
	collectionResponse, err := c.hc.Get("https://order.dominos.com.au/eStore/en/OrderTimeNowOrLater/Delivery")
	if err != nil {
		return err
	}

	if collectionResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("collection response had invalid code %v", collectionResponse.Status)
	}

	collectionDoc, err := goquery.NewDocumentFromResponse(collectionResponse)
	if err != nil {
		return err
	}

	if t := collectionDoc.Find("title").Text(); !strings.Contains(t, "Domino's Online Ordering") {
		return fmt.Errorf("collection response had invalid title: %q", t)
	}

	addressForm := make(url.Values)
	addressForm.Set("ordertimenowlater", "now")
	addressForm.Set("Customer.UnitNo", a.UnitNumber)
	addressForm.Set("Customer.StreetNo", a.StreetNumber)
	addressForm.Set("Customer.Street", a.StreetName)
	addressForm.Set("Customer.Suburb", a.Suburb)
	addressForm.Set("Customer.Postcode", a.Postcode)

	addressResponse, err := c.hc.PostForm("https://order.dominos.com.au/estore/en/DeliverySearch/AllDetails", addressForm)
	if err != nil {
		return err
	}

	if addressResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("address response had invalid code %v", addressResponse.Status)
	}

	addressDoc, err := goquery.NewDocumentFromResponse(addressResponse)
	if err != nil {
		return err
	}

	addressConfirmLink, ok := addressDoc.Find("#search-items .store-result").First().Attr("href")
	if !ok {
		return fmt.Errorf("couldn't find address confirmation link")
	}

	addressConfirmResponse, err := c.hc.Get("https://order.dominos.com.au" + addressConfirmLink)
	if err != nil {
		return err
	}

	if addressConfirmResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("address response had invalid code %v", addressConfirmResponse.StatusCode)
	}

	return nil
}

func (c *Client) ApplyVoucher(code string) error {
	f := make(url.Values)

	f.Set("voucherCode", code)
	f.Set("controllerName", "ProductMenu")
	f.Set("pageCodeProductMenu", "")
	f.Set("paymentMethod", "")
	f.Set("addFromVoucherBox", "true")

	res, err := c.hc.PostForm("https://order.dominos.com.au/eStore/en/Basket/ApplyVoucher?"+f.Encode(), f)
	if err != nil {
		return err
	}

	var d struct {
		URL              string   `json:"Url"`
		Messages         []string `json:"Messages"`
		ResponseMessages []string `json:"ResponseMessages"`
	}

	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return err
	}

	if len(d.Messages) != 0 {
		return fmt.Errorf("error: %s", d.Messages[0])
	}

	return nil
}

func (c *Client) RemoveVoucher(itemID string) error {
	f := make(url.Values)

	f.Set("orderItemId", itemID)
	f.Set("controllerName", "ProductMenu")
	f.Set("pageCodeProductMenu", "")
	f.Set("paymentMethod", "")

	res, err := c.hc.PostForm("https://order.dominos.com.au/eStore/en/Basket/RemoveVoucher?"+f.Encode(), f)
	if err != nil {
		return err
	}

	var d struct {
		URL              string   `json:"Url"`
		Messages         []string `json:"Messages"`
		ResponseMessages []string `json:"ResponseMessages"`
	}

	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return err
	}

	if len(d.Messages) != 0 {
		return fmt.Errorf("error: %s", d.Messages[0])
	}

	return nil
}

type Voucher struct {
	ItemID string
	Code   string
	Name   string
	Price  string
	Pizzas string
}

type Item struct {
	ItemID      string
	Name        string
	ProductCode string
}

type Basket struct {
	Vouchers []Voucher
	Items    []Item
}

func parseBasket(res *http.Response) (*Basket, error) {
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return nil, err
	}

	var b Basket

	doc.Find(".voucher-container").Each(func(_ int, s *goquery.Selection) {
		itemID := s.Find(".voucher-details a.remove-voucher").AttrOr("data-order-item-id", "")
		code := s.Find(".voucher-details a.remove-voucher").AttrOr("data-usr-voucher-code", "")
		name := s.Find(".voucher-details a.remove-voucher").AttrOr("data-name", "")
		price := s.Find(".voucher-details .at-voucher-price").Text()
		var pizzas string
		if a := regexp.MustCompile(`Add a Pizza \((\d+)\)`).FindStringSubmatch(s.Find(".at-voucher-fulfill").Text()); a != nil {
			pizzas = a[1]
		}
		if itemID != "" && code != "" {
			b.Vouchers = append(b.Vouchers, Voucher{
				ItemID: itemID,
				Code:   code,
				Name:   name,
				Price:  price,
				Pizzas: pizzas,
			})
		}
	})

	doc.Find(".basket-product").Each(func(_ int, s *goquery.Selection) {
		itemID := s.Find("a.remove-product").AttrOr("data-order-item-id", "")
		name := s.Find("a.remove-product").AttrOr("data-name", "")
		productCode := s.Find("a.remove-product").AttrOr("data-product-code", "")
		if itemID != "" {
			b.Items = append(b.Items, Item{
				ItemID:      itemID,
				Name:        name,
				ProductCode: productCode,
			})
		}
	})

	return &b, nil
}

func (c *Client) GetBasket() (*Basket, error) {
	res, err := c.hc.Get("https://order.dominos.com.au/eStore/en/Basket/GetBasketView")
	if err != nil {
		return nil, err
	}

	return parseBasket(res)
}

func (c *Client) RemoveItem(itemId string) (*Basket, error) {
	res, err := c.hc.Post("https://order.dominos.com.au/eStore/en/Basket/RemoveProductAndGetBasket?orderItemId="+itemId, "", nil)
	if err != nil {
		return nil, err
	}

	return parseBasket(res)
}

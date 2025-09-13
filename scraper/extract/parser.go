package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/utils"
)

type TenderParser struct{}

func NewTenderParser() *TenderParser {
	return &TenderParser{}
}

// SetupHandlers configures all the colly handlers for parsing
func (tp *TenderParser) SetupHandlers(c *colly.Collector, data *TenderData) {
	tp.setupNavigationHandler(c)
	tp.setupBasicDetailsHandler(c, data)
	tp.setupPaymentInstrumentsHandler(c, data)
	tp.setupCoverDetailsHandler(c, data)
	tp.setupTenderDocumentsHandler(c, data)
	tp.setupTenderFeeHandler(c, data)
	tp.setupEMDFeeHandler(c, data)
	tp.setupCriticalDatesHandler(c, data)
	tp.setupWorkItemHandler(c, data)
	tp.setupCorrigendumHandler(c, data)
	tp.setupCorrigendumDetailHandler(c, data)
	tp.setupTenderInvitingAuthorityHandler(c, data)
	tp.setupFallbackHandler(c, data)
}

// setupNavigationHandler handles following tender detail links
func (tp *TenderParser) setupNavigationHandler(c *colly.Collector) {
	c.OnHTML("a", func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text)
		titleAttr := strings.TrimSpace(e.Attr("title"))
		href := e.Attr("href")
		lowerText := strings.ToLower(text)
		lowerTitle := strings.ToLower(titleAttr)

		if href != "" && (strings.Contains(lowerText, "view tender details") ||
			strings.Contains(lowerTitle, "view more details") ||
			strings.Contains(href, "FrontEndTenderDetails")) {
			e.Request.Visit(e.Request.AbsoluteURL(href))
		}
	})
}

// setupBasicDetailsHandler parses the Basic Details section
func (tp *TenderParser) setupBasicDetailsHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("table", func(e *colly.HTMLElement) {
		// Detect details page
		if data.DetailsURL == "" && strings.Contains(strings.ToLower(e.DOM.Text()), "tender details") {
			data.DetailsURL = e.Request.URL.String()
		}

		// Look for Basic Details section
		sectionHead := e.DOM.Find("td.section_head").First()
		if sectionHead.Length() > 0 && strings.Contains(strings.ToLower(sectionHead.Text()), "basic details") {
			tp.parseBasicDetailsRows(e.DOM, &data.BasicDetails)
		}
	})
}

func (tp *TenderParser) parseBasicDetailsRows(dom *goquery.Selection, basic *BasicDetails) {
	dom.Find("tr.td_caption").Each(func(_ int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() == 0 {
			return
		}

		switch tds.Length() {
		case 2:
			// Simple 2-column structure: Label | Value
			label := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
			value := strings.TrimSpace(tds.Eq(1).Text())
			tp.assignBasicDetailValue(label, value, basic)
		case 4:
			// 4-column structure: Label1 | Value1 | Label2 | Value2
			label1 := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
			value1 := strings.TrimSpace(tds.Eq(1).Text())
			tp.assignBasicDetailValue(label1, value1, basic)

			label2 := strings.TrimSpace(strings.TrimSuffix(tds.Eq(2).Text(), ":"))
			value2 := strings.TrimSpace(tds.Eq(3).Text())
			tp.assignBasicDetailValue(label2, value2, basic)
		case 3:
			// Handle cases with colspan or other structures
			label := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
			value := strings.TrimSpace(tds.Eq(1).Text())
			tp.assignBasicDetailValue(label, value, basic)
		}
	})
}

func (tp *TenderParser) assignBasicDetailValue(label, value string, basic *BasicDetails) {
	labelLower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(label), ".", ""))

	switch labelLower {
	case "organisation chain":
		if basic.OrganisationChain == "" {
			basic.OrganisationChain = value
		}
	case "tender reference number":
		if basic.TenderReferenceNumber == "" {
			basic.TenderReferenceNumber = value
		}
	case "tender id":
		if basic.TenderID == "" {
			basic.TenderID = value
		}
	case "withdrawal allowed":
		if basic.WithdrawalAllowed == "" {
			basic.WithdrawalAllowed = value
		}
	case "tender type":
		if basic.TenderType == "" {
			basic.TenderType = value
		}
	case "form of contract":
		if basic.FormOfContract == "" {
			basic.FormOfContract = value
		}
	case "tender category":
		if basic.TenderCategory == "" {
			basic.TenderCategory = value
		}
	case "no of covers", "number of covers":
		if basic.NumberOfCovers == "" {
			basic.NumberOfCovers = value
		}
	case "general technical evaluation allowed":
		if basic.GeneralTechnicalEvaluationAllowed == "" {
			basic.GeneralTechnicalEvaluationAllowed = value
		}
	case "itemwise technical evaluation allowed":
		if basic.ItemWiseTechnicalEvaluationAllowed == "" {
			basic.ItemWiseTechnicalEvaluationAllowed = value
		}
	case "payment mode":
		if basic.PaymentMode == "" {
			basic.PaymentMode = value
		}
	case "is multi currency allowed for boq":
		if basic.IsMultiCurrencyAllowedForBOQ == "" {
			basic.IsMultiCurrencyAllowedForBOQ = value
		}
	case "is multi currency allowed for fee":
		if basic.IsMultiCurrencyAllowedForFee == "" {
			basic.IsMultiCurrencyAllowedForFee = value
		}
	case "allow two stage bidding":
		if basic.AllowTwoStageBidding == "" {
			basic.AllowTwoStageBidding = value
		}
	}
}

// setupPaymentInstrumentsHandler parses payment instruments
func (tp *TenderParser) setupPaymentInstrumentsHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("table#offlineInstrumentsTableView", func(e *colly.HTMLElement) {
		e.DOM.Find("tr").Each(func(_ int, s *goquery.Selection) {
			if s.HasClass("caption") {
				return
			}
			tds := s.Find("td.field_text")
			if tds.Length() >= 2 {
				serial := strings.TrimSpace(tds.Eq(0).Text())
				instr := strings.TrimSpace(tds.Eq(1).Text())
				data.PaymentInstruments = append(data.PaymentInstruments,
					utils.PaymentInstrument{SerialNo: serial, InstrumentType: instr})
			}
		})
	})
}

// setupCoverDetailsHandler parses cover details
func (tp *TenderParser) setupCoverDetailsHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("table#packetTableView", func(e *colly.HTMLElement) {
		e.DOM.Find("tr").Each(func(_ int, s *goquery.Selection) {
			if s.Find("td.section_head").Length() > 0 || s.Find("td.caption").Length() > 0 {
				return
			}
			tds := s.Find("td.field_text")
			if tds.Length() >= 4 {
				coverNo := strings.TrimSpace(tds.Eq(0).Text())
				coverType := strings.TrimSpace(tds.Eq(1).Text())
				docType := strings.TrimSpace(tds.Eq(2).Text())
				desc := strings.TrimSpace(tds.Eq(3).Text())
				data.Covers = append(data.Covers, utils.CoverInformation{
					CoverNo: coverNo, CoverType: coverType,
					DocumentType: docType, Description: desc})
			}
		})
	})
}

// setupTenderDocumentsHandler parses tender documents - FIXED
func (tp *TenderParser) setupTenderDocumentsHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if !strings.Contains(head, "tender documents") {
			return
		}

		parent := e.DOM.Closest("table")
		parent.Find("table").Each(func(_ int, table *goquery.Selection) {
			headerRow := table.Find("tr").First()
			headerText := strings.ToLower(headerRow.Text())
			if !strings.Contains(headerText, "document name") {
				return
			}

			table.Find("tr").Each(func(_ int, s *goquery.Selection) {
				if s.Find("td.caption").Length() > 0 {
					return
				}

				tds := s.Find("td")
				if tds.Length() >= 5 {
					// Work document
					doc := WorkDocument{
						SerialNo:       strings.TrimSpace(tds.Eq(0).Text()),
						DocumentType:   strings.TrimSpace(tds.Eq(1).Text()),
						DocumentName:   strings.TrimSpace(tds.Eq(2).Text()),
						Description:    strings.TrimSpace(tds.Eq(3).Text()),
						DocumentSizeKB: strings.TrimSpace(tds.Eq(4).Text()),
					}
					if doc.SerialNo != "" && doc.DocumentName != "" && !tp.workDocExists(data.WorkDocuments, doc) {
						data.WorkDocuments = append(data.WorkDocuments, doc)
					}
				} else if tds.Length() >= 4 {
					// NIT document
					doc := NITDocument{
						SerialNo:       strings.TrimSpace(tds.Eq(0).Text()),
						DocumentName:   strings.TrimSpace(tds.Eq(1).Text()),
						Description:    strings.TrimSpace(tds.Eq(2).Text()),
						DocumentSizeKB: strings.TrimSpace(tds.Eq(3).Text()),
					}
					if doc.SerialNo != "" && doc.DocumentName != "" && !tp.nitDocExists(data.NITDocuments, doc) {
						data.NITDocuments = append(data.NITDocuments, doc)
					}
				}
			})
		})
	})
}

func (tp *TenderParser) workDocExists(docs []WorkDocument, d WorkDocument) bool {
	for _, x := range docs {
		if x.SerialNo == d.SerialNo && x.DocumentName == d.DocumentName {
			return true
		}
	}
	return false
}

func (tp *TenderParser) nitDocExists(docs []NITDocument, d NITDocument) bool {
	for _, x := range docs {
		if x.SerialNo == d.SerialNo && x.DocumentName == d.DocumentName {
			return true
		}
	}
	return false
}

// setupTenderFeeHandler parses tender fee details
func (tp *TenderParser) setupTenderFeeHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if strings.Contains(head, "tender fee details") {
			// Extract total fee from header
			if m := regexp.MustCompile(`(?i)total\s*fee[^0-9]*([0-9,]+\.?[0-9]*)`).FindStringSubmatch(e.DOM.Text()); len(m) >= 2 {
				data.TenderFee.TotalFee = strings.TrimSpace(m[1])
			}

			parentTable := e.DOM.Closest("table")
			parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
				tp.parseTenderFeeRow(s, &data.TenderFee)
			})
		}
	})
}

func (tp *TenderParser) parseTenderFeeRow(s *goquery.Selection, fee *TenderFeeDetails) {
	capCell := strings.ToLower(strings.TrimSpace(s.Find("td.caption").First().Text()))

	if capCell == "tender fee in ₹" || strings.Contains(capCell, "tender fee in") {
		val := strings.TrimSpace(s.Find("td.view_list_field, td.field_text").First().Text())
		fee.TenderFee = val
	}

	tds := s.Find("td.field_text")
	if tds.Length() >= 2 && strings.Contains(strings.ToLower(strings.TrimSpace(s.Text())), "fee payable") {
		fee.FeePayableTo = strings.TrimSpace(tds.Eq(0).Text())
		fee.FeePayableAt = strings.TrimSpace(tds.Eq(1).Text())
	}

	if strings.Contains(strings.ToLower(strings.TrimSpace(s.Text())), "tender fee exemption allowed") {
		val := strings.TrimSpace(s.Find("td.field_text").First().Text())
		fee.TenderFeeExemptionAllowed = val
	}
}

// setupEMDFeeHandler parses EMD fee details
func (tp *TenderParser) setupEMDFeeHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if strings.Contains(head, "emd fee details") {
			parentTable := e.DOM.Closest("table")
			parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
				tp.parseEMDFeeRow(s, &data.EMDFee)
			})
		}
	})
}

func (tp *TenderParser) parseEMDFeeRow(s *goquery.Selection, emd *EMDFeeDetails) {
	cells := s.Find("td")
	if cells.Length() >= 2 {
		cap := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
		val := strings.TrimSpace(cells.Eq(1).Text())

		if strings.Contains(cap, "emd amount") {
			emd.EmdAmount = val
		} else if strings.Contains(cap, "fee type") {
			emd.EmdFeeType = val
		} else if strings.Contains(cap, "emd payable to") {
			emd.EmdPayableTo = val
		}
	}

	if cells.Length() >= 4 {
		cap2 := strings.ToLower(strings.TrimSpace(cells.Eq(2).Text()))
		val2 := strings.TrimSpace(cells.Eq(3).Text())

		if strings.Contains(cap2, "exemption") {
			emd.EmdExemptionAllowed = val2
		} else if strings.Contains(cap2, "percentage") {
			emd.EmdPercentage = val2
		} else if strings.Contains(cap2, "emd payable at") {
			emd.EmdPayableAt = val2
		}
	}
}

// setupCriticalDatesHandler parses critical dates - FIXED
func (tp *TenderParser) setupCriticalDatesHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if strings.Contains(head, "critical dates") {
			parentTable := e.DOM.Closest("table")
			parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
				tp.parseCriticalDatesRow(s, &data.CriticalDates)
			})
		}
	})
}

func (tp *TenderParser) parseCriticalDatesRow(s *goquery.Selection, cd *CriticalDates) {
	cells := s.Find("td")

	// Handle 4-column layout: Label1 | Value1 | Label2 | Value2
	if cells.Length() == 4 {
		cap1 := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
		val1 := strings.TrimSpace(cells.Eq(1).Text())
		tp.assignCriticalDateValue(cap1, val1, cd)

		cap2 := strings.ToLower(strings.TrimSpace(cells.Eq(2).Text()))
		val2 := strings.TrimSpace(cells.Eq(3).Text())
		tp.assignCriticalDateValue(cap2, val2, cd)
		return
	}

	// Handle 2-column layout: Label | Value
	if cells.Length() == 2 {
		cap := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
		val := strings.TrimSpace(cells.Eq(1).Text())
		tp.assignCriticalDateValue(cap, val, cd)
		return
	}

	// Fallback: try caption-based parsing
	cap := strings.ToLower(strings.TrimSpace(s.Find("td.caption").First().Text()))
	valField := s.Find("td.field_text").First()
	val := strings.TrimSpace(valField.Text())

	if cap != "" && val != "" {
		tp.assignCriticalDateValue(cap, val, cd)
	}
}

func (tp *TenderParser) assignCriticalDateValue(cap, val string, cd *CriticalDates) {
	// Clean up the caption
	cap = strings.ReplaceAll(cap, ":", "")
	cap = strings.TrimSpace(cap)

	switch cap {
	case "published date", "publish date":
		if cd.PublishedDate == "" {
			cd.PublishedDate = val
		}
	case "bid opening date":
		if cd.BidOpeningDate == "" {
			cd.BidOpeningDate = val
		}
	case "document download start date", "document download / sale start date":
		if cd.DocumentDownloadStartDate == "" {
			cd.DocumentDownloadStartDate = val
		}
	case "document download end date", "document download / sale end date":
		if cd.DocumentDownloadEndDate == "" {
			cd.DocumentDownloadEndDate = val
		}
	case "clarification start date":
		if v := strings.TrimSpace(val); v != "" && !strings.EqualFold(v, "na") && cd.ClarificationStartDate == nil {
			cd.ClarificationStartDate = &v
		}
	case "clarification end date":
		if v := strings.TrimSpace(val); v != "" && !strings.EqualFold(v, "na") && cd.ClarificationEndDate == nil {
			cd.ClarificationEndDate = &v
		}
	case "bid submission start date":
		if cd.BidSubmissionStartDate == "" {
			cd.BidSubmissionStartDate = val
		}
	case "bid submission end date":
		if cd.BidSubmissionEndDate == "" {
			cd.BidSubmissionEndDate = val
		}
	}
}

// setupWorkItemHandler parses work/item details - FIXED
func (tp *TenderParser) setupWorkItemHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if strings.Contains(head, "work /item") {
			parentTable := e.DOM.Closest("table")
			parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
				tp.parseWorkItemRow(s, &data.WorkItem)
			})
		}
	})
}

func (tp *TenderParser) parseWorkItemRow(s *goquery.Selection, work *WorkItemDetails) {
	caps := s.Find("td.caption")
	fields := s.Find("td.field_text")

	// match captions to fields in order
	for i := 0; i < caps.Length() && i < fields.Length(); i++ {
		cap := strings.ToLower(strings.TrimSpace(caps.Eq(i).Text()))
		cap = strings.TrimSuffix(cap, ":")
		cap = strings.ReplaceAll(cap, "\u00a0", "")
		val := strings.TrimSpace(fields.Eq(i).Text())

		tp.assignWorkItemValue(cap, val, work)
	}
}

func (tp *TenderParser) assignWorkItemValue(cap, val string, work *WorkItemDetails) {
	if strings.Contains(cap, "title") {
		if work.Title == "" {
			work.Title = val
		}
	}
	if strings.Contains(cap, "work description") {
		if work.Description == "" {
			work.Description = val
		}
	}
	if strings.Contains(cap, "pre qualification details") {
		if work.PreQualification == "" {
			work.PreQualification = val
		}
	}
	if strings.Contains(cap, "independent external monitor/remarks") {
		if work.IEMRemarks == "" {
			work.IEMRemarks = val
		}
	}
	if strings.Contains(cap, "tender value in ₹") {
		if work.TenderValue == "" {
			work.TenderValue = val
		}
	}
	if strings.Contains(cap, "product category") {
		if work.ProductCategory == "" {
			work.ProductCategory = val
		}
	}
	if strings.Contains(cap, "sub category") {
		if work.SubCategory == "" {
			work.SubCategory = val
		}
	}
	if strings.Contains(cap, "contract type") {
		if work.ContractType == "" {
			work.ContractType = val
		}
	}
	if strings.Contains(cap, "bid validity(days)") {
		if work.BidValidityDays == "" {
			work.BidValidityDays = val
		}
	}
	if strings.Contains(cap, "period of work(days)") {
		if work.PeriodOfWorkDays == "" {
			work.PeriodOfWorkDays = val
		}
	}
	if strings.Contains(cap, "location") {
		if work.Location == "" {
			work.Location = val
		}
	}
	if strings.Contains(cap, "pincode") {
		if work.Pincode == "" {
			work.Pincode = val
		}
	}
	if strings.Contains(cap, "pre bid meeting place") {
		if work.PreBidMeetingPlace == "" {
			work.PreBidMeetingPlace = val
		}
	}
	if strings.Contains(cap, "pre bid meeting address") {
		if work.PreBidMeetingAddress == "" {
			work.PreBidMeetingAddress = val
		}
	}
	if strings.Contains(cap, "pre bid meeting date") {
		if work.PreBidMeetingDate == "" {
			work.PreBidMeetingDate = val
		}
	}
	if strings.Contains(cap, "bid opening place") {
		if work.BidOpeningPlace == "" {
			work.BidOpeningPlace = val
		}
	}
	if strings.Contains(cap, "should allow nda tender") {
		if work.ShouldAllowNDA == "" {
			work.ShouldAllowNDA = val
		}
	}
	if strings.Contains(cap, "allow preferential bidder") {
		if work.AllowPreferentialBidder == "" {
			work.AllowPreferentialBidder = val
		}
	}
}

// setupTenderInvitingAuthorityHandler parses tender inviting authority
func (tp *TenderParser) setupTenderInvitingAuthorityHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
		if strings.EqualFold(head, "tender inviting authority") {
			parentTable := e.DOM.Closest("table")
			parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
				cells := s.Find("td")
				if cells.Length() == 2 {
					cap := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
					cap = strings.TrimSuffix(cap, ":") // handle "Name:"
					val := strings.TrimSpace(cells.Eq(1).Text())
					// fmt.Println("TenderInvitingAuthority: ", cap, val)
					switch cap {
					case "name":
						if data.TenderInvitingAuthority.Name == "" {
							data.TenderInvitingAuthority.Name = val
						}
					case "address":
						if data.TenderInvitingAuthority.Address == "" {
							data.TenderInvitingAuthority.Address = val
						}
					}

				}
			})
		}
	})
}

// setupFallbackHandler provides regex-based fallback parsing
func (tp *TenderParser) setupFallbackHandler(c *colly.Collector, data *TenderData) {
	c.OnResponse(func(r *colly.Response) {
		body := string(r.Body)

		if data.DetailsURL == "" && strings.Contains(strings.ToLower(body), "tender details") {
			data.DetailsURL = r.Request.URL.String()
		}

		if data.BasicDetails.TenderID == "" {
			re := regexp.MustCompile(`(?i)(Tender\s*(ID|Ref\.?\s*No\.?))\s*[:\-]?\s*([A-Za-z0-9\-/_.]+)`)
			if m := re.FindStringSubmatch(body); len(m) >= 4 {
				data.BasicDetails.TenderID = strings.TrimSpace(m[3])
			}
		}
	})
}

// setupCorrigendumHandler parses corrigendum list and follows links
func (tp *TenderParser) setupCorrigendumHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
		head := strings.ToLower(strings.TrimSpace(e.Text))
		if !strings.Contains(head, "corrigendum") {
			return
		}

		parent := e.DOM.Closest("table")
		parent.Find("tr").Each(func(_ int, s *goquery.Selection) {
			if s.HasClass("list_header") {
				return
			}
			tds := s.Find("td")
			if tds.Length() >= 4 && !s.HasClass("list_header") {
				rawLink := tds.Eq(3).Find("a").AttrOr("href", "")
				absLink := ""
				if rawLink != "" {
					absLink = e.Request.AbsoluteURL(rawLink)
				}

				corr := utils.Corrigendum{
					SerialNo: strings.TrimSpace(tds.Eq(0).Text()),
					Title:    strings.TrimSpace(tds.Eq(1).Text()),
					Type:     strings.TrimSpace(tds.Eq(2).Text()),
					ViewLink: absLink,
				}
				data.Corrigendum = append(data.Corrigendum, corr)

				if absLink != "" {
					e.Request.Visit(absLink)
				}
			}
		})
	})
}

func (tp *TenderParser) setupCorrigendumDetailHandler(c *colly.Collector, data *TenderData) {
	c.OnHTML("table.list_table", func(e *colly.HTMLElement) {
		// Detect corrigendum type from the info table
		corrType := ""
		e.DOM.Closest("table.page_border").Find("tr.td_caption").Each(func(_ int, s *goquery.Selection) {
			if strings.Contains(strings.ToLower(s.Text()), "corrigendum type") {
				corrType = strings.TrimSpace(s.Find("td").Last().Text())
			}
		})

		if corrType == "" {
			return // cannot proceed if we don't know type
		}

		if strings.EqualFold(corrType, "Other") {
			// Handle "Other" -> parse rows under #corrDoctable
			if id, _ := e.DOM.Attr("id"); id == "corrDoctable" {
				e.DOM.Find("tr").Each(func(_ int, tr *goquery.Selection) {
					if tr.HasClass("td_caption") {
						return
					}
					tds := tr.Find("td")
					if tds.Length() < 6 {
						return
					}
					link := tds.Eq(4).Find("a").AttrOr("href", "")
					absLink := e.Request.AbsoluteURL(link)

					detail := utils.CorrigendumDetail{
						CorrigendumNo:  strings.TrimSpace(tds.Eq(0).Text()),
						Title:          strings.TrimSpace(tds.Eq(1).Text()),
						Description:    strings.TrimSpace(tds.Eq(2).Text()),
						PublishedDate:  strings.TrimSpace(tds.Eq(3).Text()),
						DocumentName:   strings.TrimSpace(tds.Eq(4).Text()),
						DocumentLink:   absLink,
						DocumentSizeKB: strings.TrimSpace(tds.Eq(5).Text()),
					}
					tp.attachDetailToParent(data, e.Request.URL.String(), detail)
				})
			}

		} else if strings.EqualFold(corrType, "Date") {
			// Handle "Date" -> must skip Critical Dates tables
			firstCell := strings.ToLower(strings.TrimSpace(e.DOM.Find("tr").First().Text()))
			if strings.Contains(firstCell, "critical dates") {
				return // skip
			}
			e.DOM.Find("tr").Each(func(_ int, tr *goquery.Selection) {
				if tr.HasClass("td_caption") {
					return
				}
				tds := tr.Find("td")
				if tds.Length() < 5 {
					return
				}
				link := tds.Eq(3).Find("a").AttrOr("href", "")
				absLink := e.Request.AbsoluteURL(link)

				detail := utils.CorrigendumDetail{
					Title:          strings.TrimSpace(tds.Eq(0).Text()),
					Description:    strings.TrimSpace(tds.Eq(1).Text()),
					PublishedDate:  strings.TrimSpace(tds.Eq(2).Text()),
					DocumentName:   strings.TrimSpace(tds.Eq(3).Text()),
					DocumentLink:   absLink,
					DocumentSizeKB: strings.TrimSpace(tds.Eq(4).Text()),
				}
				tp.attachDetailToParent(data, e.Request.URL.String(), detail)
			})
		}
	})
}

// attachDetailToParent finds the right Corrigendum struct and appends the detail
func (tp *TenderParser) attachDetailToParent(data *TenderData, pageURL string, detail utils.CorrigendumDetail) {
	for i := range data.Corrigendum {
		if data.Corrigendum[i].ViewLink == pageURL {
			data.Corrigendum[i].Details = append(data.Corrigendum[i].Details, detail)
			break
		}
	}
}

package leia_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinRobotLiveEPAEnvironmentDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
epa_tri_request_error := nil
epa_tri_json_error := nil
epa_tri_status := 0
epa_tri_ok := false
epa_tri_content_type := ""
epa_tri_count := 0
epa_tri_doc_ctrl_num := ""
epa_tri_active_status := ""
epa_tri_facility_id := ""
epa_tri_chem_id := ""
epa_tri_form_type := ""
epa_tri_reporting_year := ""
epa_tri_chemical_name := ""
epa_tri_max_amount_code := ""
epa_tri_certif_date_signed := ""
epa_tri_received_date := ""
epa_tri_media_type := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://data.epa.gov/efservice/tri_reporting_form/REPORTING_YEAR/2023/ROWS/0:5/JSON", {
    headers: headers
    timeout: 30
})
if err != nil {
    epa_tri_request_error = err
} else {
    epa_tri_status = resp.status
    epa_tri_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        epa_tri_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            epa_tri_json_error = json_err
        } else {
            epa_tri_count = #data
            if epa_tri_count > 0 {
                row := data[1]
                epa_tri_doc_ctrl_num = row.doc_ctrl_num
                epa_tri_active_status = row.active_status
                epa_tri_facility_id = row.tri_facility_id
                epa_tri_chem_id = row.tri_chem_id
                epa_tri_form_type = row.form_type_ind
                epa_tri_reporting_year = row.reporting_year
                epa_tri_chemical_name = row.cas_chem_name
                epa_tri_max_amount_code = row.max_amount_of_chem
                epa_tri_certif_date_signed = row.certif_date_signed
                epa_tri_received_date = row.received_date
                epa_tri_media_type = row.media_type
            }
        }
    } else {
        epa_tri_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "EPA Envirofacts TRI reporting forms", "epa_tri_status", "epa_tri_request_error", "epa_tri_json_error", "epa_tri_ok")
	contentType := mustGetString(t, vm, "epa_tri_content_type")
	count := mustGetInt(t, vm, "epa_tri_count")
	docCtrlNum := mustGetString(t, vm, "epa_tri_doc_ctrl_num")
	activeStatus := mustGetString(t, vm, "epa_tri_active_status")
	facilityID := mustGetString(t, vm, "epa_tri_facility_id")
	chemID := mustGetString(t, vm, "epa_tri_chem_id")
	formType := mustGetString(t, vm, "epa_tri_form_type")
	reportingYearText := mustGetString(t, vm, "epa_tri_reporting_year")
	chemicalName := mustGetString(t, vm, "epa_tri_chemical_name")
	maxAmountCodeText := mustGetString(t, vm, "epa_tri_max_amount_code")
	certifDateSigned := mustGetString(t, vm, "epa_tri_certif_date_signed")
	receivedDate := mustGetString(t, vm, "epa_tri_received_date")
	mediaType := mustGetString(t, vm, "epa_tri_media_type")

	fmt.Printf("epa_tri content_type=%q count=%d facility=%q chem_id=%q chemical=%q year=%q max_amount_code=%q certified=%q received=%q media=%q\n", contentType, count, facilityID, chemID, chemicalName, reportingYearText, maxAmountCodeText, certifDateSigned, receivedDate, mediaType)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("EPA Envirofacts TRI Content-Type = %q, want application/json", contentType)
	}
	if count <= 0 {
		t.Fatalf("EPA Envirofacts TRI row count = %d, want > 0", count)
	}
	if docCtrlNum == "" || activeStatus == "" || facilityID == "" || chemID == "" || formType == "" ||
		chemicalName == "" || certifDateSigned == "" || receivedDate == "" || mediaType == "" {
		t.Fatalf("unexpected EPA Envirofacts TRI key fields: doc=%q active=%q facility=%q chem_id=%q form=%q chemical=%q certified=%q received=%q media=%q", docCtrlNum, activeStatus, facilityID, chemID, formType, chemicalName, certifDateSigned, receivedDate, mediaType)
	}
	if activeStatus != "0" && activeStatus != "1" {
		t.Fatalf("EPA Envirofacts TRI active_status = %q, want 0 or 1", activeStatus)
	}
	if len(facilityID) < 10 || len(chemID) < 5 {
		t.Fatalf("unexpected EPA Envirofacts TRI identifiers: facility=%q chem_id=%q", facilityID, chemID)
	}

	reportingYear, err := strconv.Atoi(reportingYearText)
	if err != nil {
		t.Fatalf("EPA Envirofacts TRI reporting_year = %q, want numeric year: %v", reportingYearText, err)
	}
	currentYear := time.Now().Year()
	if reportingYear < 1987 || reportingYear > currentYear {
		t.Fatalf("EPA Envirofacts TRI reporting_year = %d, want plausible TRI reporting year", reportingYear)
	}

	maxAmountCode, err := strconv.Atoi(maxAmountCodeText)
	if err != nil {
		t.Fatalf("EPA Envirofacts TRI max_amount_of_chem = %q, want numeric code: %v", maxAmountCodeText, err)
	}
	if maxAmountCode < 1 || maxAmountCode > 99 {
		t.Fatalf("EPA Envirofacts TRI max_amount_of_chem = %d, want plausible range code", maxAmountCode)
	}

	certifiedAt, err := time.Parse("2006-01-02 15:04:05", certifDateSigned)
	if err != nil {
		t.Fatalf("EPA Envirofacts TRI certif_date_signed = %q, want timestamp: %v", certifDateSigned, err)
	}
	receivedAt, err := time.Parse("2006-01-02 15:04:05", receivedDate)
	if err != nil {
		t.Fatalf("EPA Envirofacts TRI received_date = %q, want timestamp: %v", receivedDate, err)
	}
	if certifiedAt.Year() < reportingYear || certifiedAt.Year() > currentYear+1 || receivedAt.Year() < reportingYear || receivedAt.Year() > currentYear+1 {
		t.Fatalf("EPA Envirofacts TRI dates out of range: reporting_year=%d certified=%s received=%s", reportingYear, certifiedAt.Format(time.RFC3339), receivedAt.Format(time.RFC3339))
	}
}

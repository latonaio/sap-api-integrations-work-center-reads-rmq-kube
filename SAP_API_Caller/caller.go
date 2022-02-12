package sap_api_caller

import (
	"fmt"
	"io/ioutil"
	"net/http"
	sap_api_output_formatter "sap-api-integrations-work-center-reads-rmq-kube/SAP_API_Output_Formatter"
	"strings"
	"sync"

	"github.com/latonaio/golang-logging-library-for-sap/logger"
	"golang.org/x/xerrors"
)

type RMQOutputter interface {
	Send(sendQueue string, payload map[string]interface{}) error
}

type SAPAPICaller struct {
	baseURL      string
	apiKey       string
	outputQueues []string
	outputter    RMQOutputter
	log          *logger.Logger
}

func NewSAPAPICaller(baseUrl string, outputQueueTo []string, outputter RMQOutputter, l *logger.Logger) *SAPAPICaller {
	return &SAPAPICaller{
		baseURL:      baseUrl,
		apiKey:       GetApiKey(),
		outputQueues: outputQueueTo,
		outputter:    outputter,
		log:          l,
	}
}

func (c *SAPAPICaller) AsyncGetWorkCenter(workCenterInternalID, workCenterTypeCode string, accepter []string) {
	wg := &sync.WaitGroup{}
	wg.Add(len(accepter))
	for _, fn := range accepter {
		switch fn {
		case "WorkCenter":
			func() {
				c.WorkCenter(workCenterInternalID, workCenterTypeCode)
				wg.Done()
			}()

		default:
			wg.Done()
		}
	}

	wg.Wait()
}

func (c *SAPAPICaller) WorkCenter(workCenterInternalID, workCenterTypeCode string) {
	data, err := c.callWorkCenterSrvAPIRequirementWorkCenter(fmt.Sprintf("WorkCenterHeader(WorkCenterInternalID='%s',WorkCenterTypeCode='%s')", workCenterInternalID, workCenterTypeCode), workCenterInternalID, workCenterTypeCode)
	if err != nil {
		c.log.Error(err)
		return
	}
	err = c.outputter.Send(c.outputQueues[0], map[string]interface{}{"message": data, "function": "WorkCenter"})
	if err != nil {
		c.log.Error(err)
		return
	}
	c.log.Info(data)
}

func (c *SAPAPICaller) callWorkCenterSrvAPIRequirementWorkCenter(api, workCenterInternalID, workCenterTypeCode string) (*sap_api_output_formatter.WorkCenter, error) {
	url := strings.Join([]string{c.baseURL, "api_work_center/srvd_a2x/sap/workcenter/0001", api}, "/")
	req, _ := http.NewRequest("GET", url, nil)

	c.setHeaderAPIKeyAccept(req)
	// c.getQueryWithWorkCenter(req, workCenterInternalID, workCenterTypeCode)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, xerrors.Errorf("API request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, xerrors.Errorf("API status code %d. API request failed", resp.StatusCode)
	}

	byteArray, _ := ioutil.ReadAll(resp.Body)
	data, err := sap_api_output_formatter.ConvertToWorkCenter(byteArray, c.log)
	if err != nil {
		return nil, xerrors.Errorf("convert error: %w", err)
	}
	return data, nil
}

func (c *SAPAPICaller) setHeaderAPIKeyAccept(req *http.Request) {
	req.Header.Set("APIKey", c.apiKey)
	req.Header.Set("Accept", "application/json")
}

func (c *SAPAPICaller) getQueryWithWorkCenter(req *http.Request, workCenterInternalID, workCenterTypeCode string) {
	params := req.URL.Query()
	params.Add("$filter", fmt.Sprintf("WorkCenterInternalID eq '%s' and WorkCenterTypeCode eq '%s'", workCenterInternalID, workCenterTypeCode))
	req.URL.RawQuery = params.Encode()
}

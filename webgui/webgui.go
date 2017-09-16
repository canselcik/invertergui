/*
Copyright (c) 2015, 2017 Hendrik van Wyk
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
this list of conditions and the following disclaimer in the documentation
and/or other materials provided with the distribution.

* Neither the name of invertergui nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package webgui

import (
	"fmt"
	"github.com/hpdvanwyk/invertergui/mk2if"
	"html/template"
	"net/http"
	"sync"
	"time"
)

type WebGui struct {
	respChan chan *mk2if.Mk2Info
	stopChan chan struct{}
	template *template.Template

	muninRespChan chan muninData
	poller        mk2if.Mk2If
	wg            sync.WaitGroup

	pu *prometheusUpdater
}

func NewWebGui(source mk2if.Mk2If) *WebGui {
	w := new(WebGui)
	w.respChan = make(chan *mk2if.Mk2Info)
	w.muninRespChan = make(chan muninData)
	w.stopChan = make(chan struct{})
	var err error
	w.template, err = template.New("thegui").Parse(htmlTemplate)
	if err != nil {
		panic(err)
	}
	w.poller = source
	w.pu = newPrometheusUpdater()

	w.wg.Add(1)
	go w.dataPoll()
	return w
}

type templateInput struct {
	Error []error

	Date string

	OutCurrent string
	OutVoltage string
	OutPower   string

	InCurrent string
	InVoltage string
	InPower   string

	InMinOut string

	BatVoltage string
	BatCurrent string
	BatPower   string
	BatCharge  string

	InFreq  string
	OutFreq string

	Leds []string
}

func (w *WebGui) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	statusErr := <-w.respChan

	tmpInput := buildTemplateInput(statusErr)

	err := w.template.Execute(rw, tmpInput)
	if err != nil {
		panic(err)
	}
}

func ledName(nameInt int) string {
	name, ok := mk2if.LedNames[nameInt]
	if !ok {
		return "Unknown led"
	}
	return name
}

func buildTemplateInput(status *mk2if.Mk2Info) *templateInput {
	outPower := status.OutVoltage * status.OutCurrent
	inPower := status.InCurrent * status.InVoltage

	tmpInput := &templateInput{
		Error:      status.Errors,
		Date:       status.Timestamp.Format(time.RFC1123Z),
		OutCurrent: fmt.Sprintf("%.3f", status.OutCurrent),
		OutVoltage: fmt.Sprintf("%.3f", status.OutVoltage),
		OutPower:   fmt.Sprintf("%.3f", outPower),
		InCurrent:  fmt.Sprintf("%.3f", status.InCurrent),
		InVoltage:  fmt.Sprintf("%.3f", status.InVoltage),
		InFreq:     fmt.Sprintf("%.3f", status.InFrequency),
		OutFreq:    fmt.Sprintf("%.3f", status.OutFrequency),
		InPower:    fmt.Sprintf("%.3f", inPower),

		InMinOut: fmt.Sprintf("%.3f", inPower-outPower),

		BatCurrent: fmt.Sprintf("%.3f", status.BatCurrent),
		BatVoltage: fmt.Sprintf("%.3f", status.BatVoltage),
		BatPower:   fmt.Sprintf("%.3f", status.BatVoltage*status.BatCurrent),
		BatCharge:  fmt.Sprintf("%.3f", status.ChargeState*100),
	}
	for i := range status.LedListOn {
		tmpInput.Leds = append(tmpInput.Leds, ledName(status.LedListOn[i]))
	}
	return tmpInput
}

func (w *WebGui) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}

// dataPoll waits for data from the w.poller channel. It will send its currently stored status
// to respChan if anything reads from it.
func (w *WebGui) dataPoll() {
	pollChan := w.poller.C()
	var muninValues muninData
	s := &mk2if.Mk2Info{}
	for {
		select {
		case s = <-pollChan:
			if s.Valid {
				calcMuninValues(&muninValues, s)
				w.pu.updatePrometheus(s)
			}
		case w.respChan <- s:
		case w.muninRespChan <- muninValues:
			zeroMuninValues(&muninValues)
		case <-w.stopChan:
			w.wg.Done()
			return
		}
	}
}

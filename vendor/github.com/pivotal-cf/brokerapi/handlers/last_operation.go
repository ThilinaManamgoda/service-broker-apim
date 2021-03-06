package handlers

import (
	"net/http"

	"github.com/pivotal-cf/brokerapi/middlewares"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi/domain"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

const lastOperationLogKey = "lastOperation"

func (h APIHandler) LastOperation(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["instance_id"]
	pollDetails := domain.PollDetails{
		PlanID:        req.FormValue("plan_id"),
		ServiceID:     req.FormValue("service_id"),
		OperationData: req.FormValue("operation"),
	}

	correlationID := req.Context().Value(middlewares.CorrelationIDKey).(string)

	logger := h.logger.Session(lastOperationLogKey, lager.Data{
		instanceIDLogKey:             instanceID,
		middlewares.CorrelationIDKey: correlationID,
	})

	logger.Info("starting-check-for-operation")

	lastOperation, err := h.serviceBroker.LastOperation(req.Context(), instanceID, pollDetails)

	if err != nil {
		switch err := err.(type) {
		case *apiresponses.FailureResponse:
			logger.Error(err.LoggerAction(), err)
			h.respond(w, err.ValidatedStatusCode(logger), err.ErrorResponse())
		default:
			logger.Error(unknownErrorKey, err)
			h.respond(w, http.StatusInternalServerError, apiresponses.ErrorResponse{
				Description: err.Error(),
			})
		}
		return
	}

	logger.WithData(lager.Data{"state": lastOperation.State}).Info("done-check-for-operation")

	lastOperationResponse := apiresponses.LastOperationResponse{
		State:       lastOperation.State,
		Description: lastOperation.Description,
	}

	h.respond(w, http.StatusOK, lastOperationResponse)
}

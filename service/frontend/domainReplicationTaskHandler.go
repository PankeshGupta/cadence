// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package frontend

import (
	"errors"

	"github.com/uber-common/bark"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/persistence"
)

var (
	// ErrInvalidDomainStatus is the error to indicate invalid domain status
	ErrInvalidDomainStatus = errors.New("invalid domain status attribute")
)

// NOTE: the counterpart of domain replication receiving logic is in service/worker package

type (
	// DomainReplicator is the interface which can replicate the domain
	DomainReplicator interface {
		HandleTransmissionTask(domainOperation replicator.DomainOperation, info *persistence.DomainInfo,
			config *persistence.DomainConfig, replicationConfig *persistence.DomainReplicationConfig,
			configVersion int64, failoverVersion int64) error
	}

	domainReplicatorImpl struct {
		kafka  messaging.Producer
		logger bark.Logger
	}
)

// NewDomainReplicator create a new instance odf domain replicator
func NewDomainReplicator(kafka messaging.Producer, logger bark.Logger) DomainReplicator {
	return &domainReplicatorImpl{
		kafka:  kafka,
		logger: logger,
	}
}

// HandleTransmissionTask handle transmission of the domain replication task
func (domainReplicator *domainReplicatorImpl) HandleTransmissionTask(domainOperation replicator.DomainOperation,
	info *persistence.DomainInfo, config *persistence.DomainConfig, replicationConfig *persistence.DomainReplicationConfig,
	configVersion int64, failoverVersion int64) error {
	status, err := domainReplicator.convertDomainStatusToThrift(info.Status)
	if err != nil {
		return err
	}

	taskType := replicator.ReplicationTaskTypeDomain
	task := &replicator.DomainTaskAttributes{
		DomainOperation: &domainOperation,
		ID:              common.StringPtr(info.ID),
		Info: &shared.DomainInfo{
			Name:        common.StringPtr(info.Name),
			Status:      status,
			Description: common.StringPtr(info.Description),
			OwnerEmail:  common.StringPtr(info.OwnerEmail),
			Data:        info.Data,
		},
		Config: &shared.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(config.Retention),
			EmitMetric:                             common.BoolPtr(config.EmitMetric),
		},
		ReplicationConfig: &shared.DomainReplicationConfiguration{
			ActiveClusterName: common.StringPtr(replicationConfig.ActiveClusterName),
			Clusters:          domainReplicator.convertClusterReplicationConfigToThrift(replicationConfig.Clusters),
		},
		ConfigVersion:   common.Int64Ptr(configVersion),
		FailoverVersion: common.Int64Ptr(failoverVersion),
	}

	return domainReplicator.kafka.Publish(&replicator.ReplicationTask{
		TaskType:             &taskType,
		DomainTaskAttributes: task,
	})
}

func (domainReplicator *domainReplicatorImpl) convertClusterReplicationConfigToThrift(
	input []*persistence.ClusterReplicationConfig) []*shared.ClusterReplicationConfiguration {
	output := []*shared.ClusterReplicationConfiguration{}
	for _, cluster := range input {
		clusterName := common.StringPtr(cluster.ClusterName)
		output = append(output, &shared.ClusterReplicationConfiguration{ClusterName: clusterName})
	}
	return output
}

func (domainReplicator *domainReplicatorImpl) convertDomainStatusToThrift(input int) (*shared.DomainStatus, error) {
	switch input {
	case persistence.DomainStatusRegistered:
		output := shared.DomainStatusRegistered
		return &output, nil
	case persistence.DomainStatusDeprecated:
		output := shared.DomainStatusDeprecated
		return &output, nil
	default:
		return nil, ErrInvalidDomainStatus
	}
}

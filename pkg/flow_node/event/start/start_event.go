// Copyright (c) 2021 Aree Enterprises, Inc. and Contributors
// Use of this software is governed by the Business Source License
// included in the file LICENSE
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/LICENSE-Apache-2.0

package start

import (
	"sync"

	"bpxe.org/pkg/bpmn"
	"bpxe.org/pkg/event"
	"bpxe.org/pkg/flow"
	"bpxe.org/pkg/flow/flow_interface"
	"bpxe.org/pkg/flow_node"
	"bpxe.org/pkg/id"
	"bpxe.org/pkg/tracing"
)

type message interface {
	message()
}

type nextActionMessage struct {
	response chan flow_node.Action
}

func (m nextActionMessage) message() {}

type Node struct {
	flow_node.T
	element       *bpmn.StartEvent
	runnerChannel chan message
	activated     bool
	idGenerator   id.Generator
}

func New(process *bpmn.Process,
	definitions *bpmn.Definitions,
	startEvent *bpmn.StartEvent,
	eventIngress event.ProcessEventConsumer,
	eventEgress event.ProcessEventSource,
	tracer *tracing.Tracer,
	flowNodeMapping *flow_node.FlowNodeMapping,
	flowWaitGroup *sync.WaitGroup,
	idGenerator id.Generator,
) (node *Node, err error) {
	flowNode, err := flow_node.New(process,
		definitions,
		&startEvent.FlowNode,
		eventIngress, eventEgress,
		tracer, flowNodeMapping,
		flowWaitGroup)
	if err != nil {
		return
	}
	node = &Node{
		T:             *flowNode,
		element:       startEvent,
		runnerChannel: make(chan message, len(flowNode.Incoming)*2+1),
		activated:     false,
		idGenerator:   idGenerator,
	}
	go node.runner()
	err = node.EventEgress.RegisterProcessEventConsumer(node)
	if err != nil {
		return
	}
	return
}

func (node *Node) runner() {
	for {
		msg := <-node.runnerChannel
		switch m := msg.(type) {
		case nextActionMessage:
			if !node.activated {
				node.activated = true
				m.response <- flow_node.FlowAction{SequenceFlows: flow_node.AllSequenceFlows(&node.Outgoing)}
			} else {
				m.response <- flow_node.CompleteAction{}
			}
		default:
		}
	}
}

func (node *Node) ConsumeProcessEvent(
	ev event.ProcessEvent,
) (result event.ConsumptionResult, err error) {
	switch ev.(type) {
	case *event.StartEvent:
		newFlow := flow.New(node.T.Definitions, node, node.T.Tracer,
			node.T.FlowNodeMapping, node.T.FlowWaitGroup, node.idGenerator, nil)
		newFlow.Start()
	default:
	}
	result = event.Consumed
	return
}

func (node *Node) NextAction(flow_interface.T) chan flow_node.Action {
	response := make(chan flow_node.Action)
	node.runnerChannel <- nextActionMessage{response: response}
	return response
}

func (node *Node) Element() bpmn.FlowNodeInterface {
	return node.element
}
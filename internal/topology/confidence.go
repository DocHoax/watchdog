package topology

// CalculateConfidence determines the confidence level of a dependency relationship
// based on evidence quality and observation source.
func CalculateConfidence(evidence []string, source string) Confidence {
	if len(evidence) == 0 {
		return ConfidenceLow
	}

	highCount := 0
	for _, ev := range evidence {
		switch ev {
		case "docker_link", "docker_network", "k8s_service_binding", "config_declared", "process_socket_exact":
			highCount++
		case "port_listening_match", "process_parent_child", "host_container_mapping":
			highCount++
		case "network_connection_established":
			highCount++
		}
	}

	if highCount >= 2 || (highCount >= 1 && (source == "docker" || source == "declared" || source == "k8s")) {
		return ConfidenceHigh
	}
	if highCount == 1 || len(evidence) >= 2 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

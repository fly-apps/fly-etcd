package flyetcd

import (
	"context"
)

func ClusterBootstrapped(client *Client) bool {
	_, err := client.MemberList(context.TODO())
	if err != nil {
		return false
	}
	return true
}

// ClusterBootrapped will determine whether the cluster has been bootstrapped or not by
// simply asking the other members in the network what their bootstrap status is.
// func ClusterBootstrapped() (bool, error) {
// 	privateIp, err := privnet.PrivateIPv6()
// 	if err != nil {
// 		return false, err
// 	}

// 	addrs, err := privnet.AllPeers(context.TODO(), os.Getenv("FLY_APP_NAME"))
// 	if err != nil {
// 		return false, err
// 	}

// 	for _, addr := range addrs {
// 		fmt.Printf("Evaluating: %s. My ip: %s\n", addr.String(), privateIp.String())

// 		if addr.String() == privateIp.String() {
// 			fmt.Printf("Skipping.\n")
// 			continue
// 		}

// 		url := fmt.Sprintf("http://[%s]:5000/bootstrapped", addr.String())
// 		// Get request will fail if endpoint isn't up yet.
// 		resp, err := http.Get(url)
// 		if err != nil {
// 			continue
// 		}

// 		body, err := ioutil.ReadAll(resp.Body)
// 		if err != nil {
// 			return false, err
// 		}
// 		result := strings.TrimSuffix(string(body), "\n")

// 		fmt.Printf("Member: %s is reporting a bootstrap status of: %q \n", addr.String(), result)
// 		//Convert the body to type string
// 		if result == "true" {
// 			return true, nil
// 		}
// 	}

// 	return false, nil
// }

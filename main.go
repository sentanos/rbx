package main

import (
	"fmt"
	"github.com/sentanos/rbx/user"
	"log"
)

func main() {
	cookie := `_|WARNING:-DO-NOT-SHARE-THIS.--Sharing-this-will-allow-someone-to-log-in-as-you-and-to-steal-your-ROBUX-and-items.|_D6A57EF4BC4D02B78C6530D9AC9511A826CBE53E5BCF207299AEB9008890B39D24622AB7CB8AD1A1378A609B01257F41BE1E2134944F6C656BDC5830AB4C7A4ACA5400E9670F2F0DAB75E45F85D53374B9A0BA43B778CFAB7D9A4784E39DEB886543A1E2BB348BE19AD7F06081F92CECEF45CC693B1E2C20A9D1115E40FE5A19FB0100A3883FB864C015C8CFB30E09D82003F74044F82B93E9BBACA85C06225F85FA4A27E1755FEC5C5C0B82061B14212389F29353CE0EC1427A7836792B6D708433435E03FD5751AE09D3183C0F45C8903848F95374492F46200C1DECAE9187C22DF6B09C42A7246D81363FEA8FAE6E1C5C6790F6309114E38A9AADE9541A4812883E2778B51A18BB571612FAC116DAF54A60E37913DB0AA0DD147BBE710E83E48AF919EEEF66D66C49AD33FCE16B26DA0BF796`
	account, err := user.LoginWithCookie(cookie)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	id, name, err := account.Status()
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	fmt.Println(id, name)
	sales, errs, cancel := account.TrackSales(-1)
	for {
		select {
		case transaction := <-sales:
			fmt.Println(transaction)
		case err := <-errs:
			fmt.Println(err)
			cancel <- true
			return
		}
	}
	// data := `<roblox></roblox>`
	// asset, assetVersion, err := account.UploadModel(strings.NewReader(data),
	// 	user.ModelOptions{
	// 		Name:          "Test",
	// 		Description:   "Testing",
	// 		CopyLocked:    false,
	// 		AllowComments: false,
	// 		GroupID:       -1,
	// 	})
	// if err != nil {
	// 	log.Fatalf("%v\n", err)
	// }
	// fmt.Println(asset, assetVersion)
}

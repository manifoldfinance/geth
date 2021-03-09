// Copyright 2019 The multi-geth Authors
// This file is part of the multi-geth library.
//
// The multi-geth library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The multi-geth library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the multi-geth library. If not, see <http://www.gnu.org/licenses/>.
package params

// ClassicBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the Ethereum Classic network.
var ClassicBootnodes = []string{
	"enode://158ac5a4817265d0d8b977660b3dbe9abee5694ed212f7091cbf784ddf47623ed015e1cb54594d10c1c46118747ddabe86ebf569cf24ae91f2daa0f1adaae390@159.203.56.33:30303", // @whilei

	"enode://942bf2f0754972391467765be1d98206926fc8ad0be8a49cd65e1730420c37fa63355bddb0ae5faa1d3505a2edcf8fad1cf00f3c179e244f047ec3a3ba5dacd7@176.9.51.216:30355", // @q9f ceibo
	"enode://0b0e09d6756b672ac6a8b70895da4fb25090b939578935d4a897497ffaa205e019e068e1ae24ac10d52fa9b8ddb82840d5d990534201a4ad859ee12cb5c91e82@176.9.51.216:30365", // @q9f ceibo

	"enode://efd48ad0879eeb7f9cb5e50f33f7bc21e805a72e90361f145baaa22dd75d111e7cd9c93f1b7060dcb30aa1b3e620269336dbf32339fea4c18925a4c15fe642df@18.205.66.229:30303",
	"enode://5fbfb426fbb46f8b8c1bd3dd140f5b511da558cd37d60844b525909ab82e13a25ee722293c829e52cb65c2305b1637fa9a2ea4d6634a224d5f400bfe244ac0de@162.243.55.45:30303",
	"enode://6dd3ac8147fa82e46837ec8c3223d69ac24bcdbab04b036a3705c14f3a02e968f7f1adfcdb002aacec2db46e625c04bf8b5a1f85bb2d40a479b3cc9d45a444af@104.237.131.102:30303",
	"enode://b9e893ea9cb4537f4fed154233005ae61b441cd0ecd980136138c304fefac194c25a16b73dac05fc66a4198d0c15dd0f33af99b411882c68a019dfa6bb703b9d@18.130.93.66:30303",
	"enode://3fe9705a02487baea45c1ffebfa4d84819f5f1e68a0dbc18031553242a6a08e39499b61e361a52c2a92f9553efd63763f6fdd34692be0d4ba6823bb2fc346009@178.62.238.75:30303",
	"enode://d50facc65e46bda6ff594b6e95491efa16e067de41ae96571d9f3cb853d538c44864496fa5e4df10115f02bbbaf47853f932e110a44c89227da7c30e96840596@188.166.163.187:30303",
	"enode://a0d5c589dc02d008fe4237da9877a5f1daedee0227ab612677eabe323520f003eb5e311af335de9f7964c2092bbc2b3b7ab1cce5a074d8346959f0868b4e366e@46.101.78.44:30303",
	"enode://c071d96b0c0f13006feae3977fb1b3c2f62caedf643df9a3655bc1b60f777f05e69a4e58bf3547bb299210092764c56df1e08380e91265baa845dca8bc0a71da@68.183.99.5:30303",
	"enode://83b33409349ffa25e150555f7b4f8deebc68f3d34d782129dc3c8ba07b880c209310a4191e1725f2f6bef59bce9452d821111eaa786deab08a7e6551fca41f4f@206.189.68.191:30303",
	"enode://0daae2a30f2c73b0b257746587136efb8e3479496f7ea1e943eeb9a663b72dd04582f699f7010ee02c57fc45d1f09568c20a9050ff937f9139e2973ddd98b87b@159.89.169.103:30303",
	"enode://50808461dd73b3d70537e4c1e5fafd1132b3a90f998399af9205f8889987d62096d4e853813562dd43e7270a71c9d9d4e4dd73a534fdb22fbac98c389c1a7362@178.128.55.119:30303",
	"enode://5cd218959f8263bc3721d7789070806b0adff1a0ed3f95ec886fb469f9362c7507e3b32b256550b9a7964a23a938e8d42d45a0c34b332bfebc54b29081e83b93@35.187.57.94:30303",

	"enode://66498ac935f3f54d873de4719bf2d6d61e0c74dd173b547531325bcef331480f9bedece91099810971c8567eeb1ae9f6954b013c47c6dc51355bbbbae65a8c16@54.148.165.1:30303",    // ETC Labs
	"enode://73e74ce7426a17aa2d8b5bb64d796ec7dc4dcee2af9bbdd4434394d1e4e52e650b9e39202435fca29ce65c242fd6e59b93ed2cf37f04b645fb23e306273816ad@54.148.165.1:30304",    // ETC Labs
	"enode://f1d4bde4cc8df2b8ce46194247450e54b9ba9e69410a30974a894a49273b10c069e7a1ff0ffd4921cb23af9f903d8257ed0133317850e670f2792f20e771a69e@123.57.29.99:30303",    //ETC Labs
	"enode://7a7768a1603b6714acd16f646b84a5aff418869b5715fa606172d19e4e7d719699448b7707e30c7b59470b4fb8b38a020135641304861483a7dc3ba988e98490@47.108.52.30:30303",    //ETC Labs
	"enode://903fceb08534c13fe1114b0c753a89c6c2ec50f973a85d308456c46721f8904b1cd47df9231114299be84e1e954db06552f791a0facbd86359aa9fd321d2ef50@47.240.106.205:30303",  //ETC Labs
	"enode://c7ec057ad9450d2d5c4002e49e53e1d90142acd54128c89e794d11401de026b91aa0e84e3f679fb6b47f7940e08b5e7d21d8178c5fa6ba0f36971098cb566ea6@101.133.229.66:30303",  //ETC Labs
	"enode://70b74fef51aa4f36330cc52ac04f16d38e1838f531f58bbc88365ca5fd4a3da6e8ec32316c43f79b157d0575faf53064fd925644d0f620b2b201b028c2b410d0@47.115.150.90:30303",   //ETC Labs
	"enode://fa64d1fcb2d4cd1d1606cb940ea2b69fee7dc6c7a85ac4ad05154df1e9ae9616a6a0fa67a59cb15f79346408efa5a4efeba1e5993ddbf4b5cedbda27644a61cf@47.91.30.48:30303",     //ETC Labs
	"enode://8e73168affd8d445edda09c561d607081ca5d7963317caae2702f701eb6546b06948b7f8687a795de576f6a5f33c44828e25a90aa63de18db380a11e660dd06f@159.203.37.80:30303",   // ETC Cooperative
	"enode://1d9aef3350a712ca90cabdf7b94a6f29e7df390c083d39d739c58cdba8708c4fbeb7784e2c8ae7a9086141e31c7df193c5a247d40264d040e6ac02d316c2fcfb@67.207.93.100:30303",   // ETC Cooperative
	"enode://506e4f07e736a6415aac813d5f2365f6359a9877844b02799819d185571908188c8ddd855897c814583f5f36f30e5cb76472e13108e6ecb0fc1901d71f0df8e2@128.199.160.131:30303", // ETC Cooperative
	"enode://84e70a981a436efd7d6ae80b5d7ecb82636de77029635a2fb25ccf5d02106172d1cafdd7d8171a136d3fd545e5df94f4d63299eda7f8aec79967aa568ddaa71e@165.22.202.207:30303",  // ETC Cooperative
	"enode://6ec7ac618a7147d8b0e41bc1e63080abfd56a48f50f06095c112e30f72cd5eee262b06aa168cb46ab470d56885f842a1ae44a3714bfb029ced83dab461852062@164.90.218.200:30303",  // ETC Cooperative
	"enode://feba6c4bd757efce4fec0fa5775780d2c209e86231553a72738c2c4e95d0cfd7ac2189f6d30d3c059dd25d93ea060ca28f9936b564f7d04c4270bb60e8e1487b@178.128.171.230:30303", // ETC Cooperative
	"enode://c0b698f9fb1c5dd3b348c46dffbc524cebbfa48fd03c52523892d43d31d5fabacf4e27f215f5647c96190567975e423a5708cbdcd6441da2705bd85533e78eca@209.97.136.89:30303",   // ETC Cooperative
	"enode://37b20dc1ca0ba428dc07f4bf87c548aa57b53e6883c607a041c88cc0f72c27ef3487b0e94f50e787fff8af5e2baf029cd6105461aa8ec250ccae93d6972107a9@134.209.30.110:30303",  // ETC Cooperative
}

var dnsPrefixETC = "enrtree://AJE62Q4DUX4QMMXEHCSSCSC65TDHZYSMONSD64P3WULVLSF6MRQ3K@"

var ClassicDNSNetwork1 = dnsPrefixETC + "all.classic.blockd.info"

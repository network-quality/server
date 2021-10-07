class Salty {
  var saltySizes = [
    "1b": 1,
    "256b": 256,
    "1k": 1 * 1024,
    "2k": 2 * 1024,
    "10k": 10 * 1024,
    "50k": 50 * 1024,
    "100k": 100 * 1024,
    "250k": 250 * 1024,
    "500k": 500 * 1024,
    // mb's added below
    "1g": 1024 * 1024 * 1024,
    "5g": 5 * 1024 * 1024 * 1024,
    "10g": 10 * 1024 * 1024 * 1024,
  ]

  var sizeNames: [Int: String] = [:]
  var sizeList: [Int] = []

  init() {
    for mb in [1, 2, 5, 10, 20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 250, 500] {
      saltySizes[String(mb) + "m"] = mb * 1024 * 1024
    }

    for (name, size) in saltySizes {
      sizeList.append(size)
      sizeNames[size] = name
    }
    sizeList.sort()
  }
}

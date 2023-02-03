//
//  NetworkQualityConfig.swift
//  networkQualityd
//
//  Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

import Foundation


/*
   {
   "version": 1,
   "urls": {
   "small_https_download_url": "https://networkquality.example.com/api/v1/gm/small",
   "large_https_download_url": "https://networkquality.example.com/api/v1/gm/large",
   "https_upload_url": "https://networkquality.example.com/api/v1/gm/slurp",
   "small_download_url": "https://networkquality.example.com/api/v1/gm/small",
   "large_download_url": "https://networkquality.example.com/api/v1/gm/large",
   "upload_url": "https://networkquality.example.com/api/v1/gm/slurp"
   }
 }
 */

class NetworkQualityURLs: Codable {
  var small_https_download_url: String
  var large_https_download_url: String
  var https_upload_url: String
  var small_download_url: String
  var large_download_url: String
  var upload_url: String

  init(domainPort:String) {
    self.small_https_download_url = NetworkQualityURLs.url(domainPort: domainPort, type: "small")
    self.large_https_download_url = NetworkQualityURLs.url(domainPort: domainPort, type: "large")
    self.https_upload_url = NetworkQualityURLs.url(domainPort: domainPort, type: "slurp")
    self.small_download_url = NetworkQualityURLs.url(domainPort: domainPort, type: "small")
    self.large_download_url = NetworkQualityURLs.url(domainPort: domainPort, type: "large")
    self.upload_url = NetworkQualityURLs.url(domainPort: domainPort, type: "slurp")
  }

  class func url(domainPort:String, type:String) -> String {
    return "https://\(domainPort)/\(type)"
  }
  class func urlReal(domainPort:String, type:String) -> String {
    return "https://\(domainPort)/\(type)"
  }
}

struct NetworkQualityConfig: Codable {
  var version: Int = 1
  var urls: NetworkQualityURLs

  init(domainPort:String) {
    self.urls = NetworkQualityURLs(domainPort: domainPort)
  }

  /// Encodes this value into the given encoder.
  public func encode(to encoder: Encoder) throws {
    var container = encoder.container(keyedBy: CodingKeys.self)
    try container.encode(self.urls, forKey: .urls)
    try container.encode(self.version, forKey: .version)
  }
}

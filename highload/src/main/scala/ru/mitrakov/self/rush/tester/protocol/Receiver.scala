package ru.mitrakov.self.rush.tester.protocol

import SwUDP._

import scala.collection.mutable
import akka.actor.ActorRef
import ru.mitrakov.self.rush.tester.Network


/**
  * Created by mitrakov on 10.07.2017.
  */
class Receiver(network: Network, listener: ActorRef) {
  private var expected = 0
  private var connected = false
  private val buffer = mutable.Map.empty[Int, List[Int]]

  def onMsg(id: Int, crcid: Int, msg: List[Int]): Unit = synchronized {
    if (id == SYN) {
      network send List(id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF)
      buffer.clear()
      expected = next(id)
      connected = true
      println("receiver connected")
    } else if (connected) {
      network send List(id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF)
      if (id == expected) {
        listener ! SwUDPUnpacked(crcid, msg)
        expected = next(id)
        accept(crcid)
      } else if (after(id, expected)) {
        buffer(id) = msg
      }
    } else network send List(ERRACK, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF)
  }

  private def accept(crcid: Int): Unit = {
    buffer get expected foreach { msg =>
      listener ! SwUDPUnpacked(crcid, msg)
      buffer -= expected
      expected = next(expected)
      accept(crcid)
    }
  }
}

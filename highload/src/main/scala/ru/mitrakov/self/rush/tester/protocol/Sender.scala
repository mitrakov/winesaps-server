package ru.mitrakov.self.rush.tester.protocol

import SwUDP._
import ru.mitrakov.self.rush.tester.Network

import scala.math._
import scala.collection.mutable


/**
  * Created by mitrakov on 10.07.2017.
  */
class Sender(network: Network) {
  private var id = 0
  private var expectedAck = 0
  private var connected = false
  private var srtt = 0
  private var totalTicks = 0
  private var crcid = 0
  private val buffer = mutable.Map.empty[Int, Item]

  def connect(crc_id: Int): Unit = synchronized {
    crcid = crc_id
    id = SYN
    expectedAck = SYN
    srtt = DEFAULT_SRTT
    totalTicks = 0
    connected = false
    buffer.clear()
    val start = List(id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF, 0xFD)
    buffer(id) = new Item(totalTicks, start)
    //println(s"Send: $start")
    network send start
  }

  def send(msg: List[Int]): Unit = synchronized {
    if (connected) {
      id = next(id)
      val fullMsg = List(id, (crcid >> 24) & 0xFF, (crcid >> 16) & 0xFF, (crcid >> 8) & 0xFF, crcid & 0xFF) ++ msg
      buffer(id) = new Item(totalTicks, fullMsg)
      //println(s"Send: $fullMsg")
      network send fullMsg
    } //else println("error: not connected")
  }

  def onAck(ack: Int): Unit = synchronized {
    buffer get ack foreach { item =>
      item.ack = true
      if (ack == expectedAck) {
        val rtt = totalTicks - item.startRtt + 1
        val newSrtt = RC*srtt + (1-RC)*rtt
        srtt = min(max(newSrtt.toInt, MIN_SRTT), MAX_SRTT)
        accept()
      }
    }
    if (ack == SYN) {
      connected = true
      println("sender connected")
    } else if (ack == ERRACK) {
      connected = false
      buffer.clear()
      println("connection failed")
    }
  }

  private def accept(): Unit = {
    buffer get expectedAck foreach { item =>
      if (item.ack) {
        buffer -= expectedAck
        expectedAck = next(expectedAck)
        accept()
      }
    }
  }

  private[protocol] def trigger(): Unit = synchronized {
    totalTicks = totalTicks + 1
    val i = expectedAck
    buffer get i foreach { item =>
      if (!item.ack) {
        if (item.attempt > MAX_ATTEMPTS) {
          connected = false
          buffer.clear()
          println("connection failed")
          return
        } else if (item.ticks == item.nextRepeat) {
          item.attempt = item.attempt+1
          item.nextRepeat += (AC*srtt*item.attempt).toInt
          if (item.attempt > 1) {
            item.startRtt = totalTicks
            //println(s"Sendd ${item.msg}, attempt = ${item.attempt}, ticks = ${item.ticks}, SRTT = $srtt")
            network send item.msg
          }
        }
        item.ticks = item.ticks+1
      }
    }
  }
}

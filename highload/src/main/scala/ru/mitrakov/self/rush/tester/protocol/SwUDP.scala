package ru.mitrakov.self.rush.tester.protocol

import SwUDP._
import akka.actor._
import ru.mitrakov.self.rush.tester.Network

import scala.annotation.tailrec
import scala.collection.mutable
import scala.concurrent.duration._
import scala.language.postfixOps
import scala.concurrent.ExecutionContext.Implicits._

/**
  * Created by mitrakov on 10.07.2017.
  */
class SwUDP(network: Network) extends Actor {
  private val senders = mutable.Map.empty[Int, Sender]
  private val receivers = mutable.Map.empty[Int, Receiver]
  private var listener: ActorRef = _

  context.system.scheduler.schedule(PERIOD milliseconds, PERIOD milliseconds, new Runnable {
    override def run(): Unit = synchronized {
      senders.values foreach {_.trigger()}
    }
  })

  override def receive: Receive = {
    case SwUDPConnect(crcId) => connect(crcId)
    case SwUDPSend(crcId, msg) => send(crcId, msg)
    case SwUDPReceived(msg) => onReceived(msg)
    case x: ActorRef => listener = x
    case x => println(s"[SwUDP] Unknown message $x")
  }

  private def connect(crcId: Int): Unit = synchronized {
    getSender(crcId) connect crcId
  }

  private def send(crcId: Int, msg: List[Int]): Unit = synchronized {
    getSender(crcId) send msg
  }

  private def onReceived(msg: List[Int]): Unit = synchronized {
    msg match {
      case id :: crcid1 :: crcid2 :: crcid3 :: crcid4 :: Nil =>
        val crcId = (crcid1 << 24) | (crcid2 << 16) | (crcid3 << 8) | crcid4
        getSender(crcId).onAck(id)
      case id :: crcid1 :: crcid2 :: crcid3 :: crcid4 :: tail =>
        val crcId = (crcid1 << 24) | (crcid2 << 16) | (crcid3 << 8) | crcid4
        getReceiver(crcId).onMsg(id, crcId, tail)
      case _ => println(s"[SwUDP] Unparsed message $msg")
    }
  }

  private def getSender(crcId: Int) = senders get crcId match {
    case Some(s) => s
    case None =>
      val s = new Sender(network)
      senders += crcId -> s
      s
  }

  private def getReceiver(crcId: Int) = receivers get crcId match {
    case Some(r) => r
    case None =>
      val r = new Receiver(network, listener)
      receivers += crcId -> r
      r
  }
}

object SwUDP {
  private[protocol] val N = 256
  private[protocol] val SYN = 0
  private[protocol] val ERRACK = 1
  private[protocol] val MAX_ATTEMPTS = 8
  private[protocol] val PERIOD = 10
  private[protocol] val MIN_SRTT = 2
  private[protocol] val DEFAULT_SRTT = 6
  private[protocol] val MAX_SRTT = 12
  private[protocol] val RC =  .8
  private[protocol] val AC = 2.2

  case class SwUDPConnect(crcId: Int)
  case class SwUDPSend(crcId: Int, msg: List[Int])
  case class SwUDPReceived(msg: List[Int])
  case class SwUDPUnpacked(crcId: Int, msg: List[Int])

  sealed class Item(var startRtt: Int, val msg: List[Int]) {
    var ack = false
    var attempt = 0
    var ticks = 0
    var nextRepeat = 0
  }

  @tailrec
  def next(n: Int): Int = {
    val result = (n+1) % N
    val ok = result != SYN && result != ERRACK
    if (ok) result else next(result)
  }

  def after(x: Int, y: Int): Boolean = (y-x+N) % N > N/2
}
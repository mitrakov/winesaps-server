package ru.mitrakov.self.rush.tester

import akka.actor.Actor

import scala.annotation.tailrec

/**
  * Created by mitrakov on 05.07.2017.
  */
class Splitter extends Actor {
  override def receive: Receive = {
    case FullMessage(crcId, data) => split(crcId, data)
    case x => println(s"[Splitter] Unknown message: $x")
  }

  private def split(crcId: Int, data: List[Int]): Unit = {
    if (data.length > 6) {
      val sid = data.head*256 + data(1)
      val token = (data(2) << 24) | (data(3) << 16) | (data(4) << 8) | data(5)
      val flags = data(6)
      splitOne(crcId, sid, token, flags, data.drop(7))
    }
  }

  @tailrec
  private def splitOne(crcId: Int, sid: Int, token: Int, flags: Int, lst: List[Int]): Unit = {
    if (lst.length > 2) {
      val length = lst.head*256 + lst(1)
      val msg = lst.slice(2, 2 + length)
      sender() ! SingleMessage(crcId, sid, token, msg)
      splitOne(crcId, sid, token, flags, lst.drop(2 + length))
    }
  }
}

package ru.mitrakov.self.rush.tester

import java.net.DatagramPacket

import akka.actor._
import ru.mitrakov.self.rush.tester.Agent._
import ru.mitrakov.self.rush.tester.protocol.SwUDP._
import scala.math._
import scala.compat.Platform

/**
  * Created by mitrakov on 04.07.2017.
  */
class Worker(protocol: ActorRef) extends Actor {
  private val splitter = context.actorOf(Props[Splitter])
  private val parser = context.actorOf(Props[Parser])

  override def receive: Receive = {
    case Connect(crcId) =>
      protocol ! SwUDPConnect(crcId)                                      // protocol will send CONNECT to network
    case Go(crcId) =>
      getNextMessage(crcId) foreach {parser ! _}                          // it will return OutputMessage back
    case OutputMessage(crcId, lst) =>
      protocol ! SwUDPSend(crcId, lst)                                    // protocol will send a result to network
    case packet: DatagramPacket =>
      protocol ! SwUDPReceived(extract(packet.getData, packet.getLength)) // it will return SwUDPUnpacked back
    case SwUDPUnpacked(crcId, msg) =>
      splitter ! FullMessage(crcId, msg)                                  // it will return N SingleMessages back
    case x@SingleMessage(_, _, _, _) =>
      parser ! x                                                          // it will return Message back
    case Start(crcId, name) =>
      getAgent(crcId).nextMessage = Some(SignIn(crcId, 0, 0, 1, name, "0cbc6611f5540bd0809a388dc95a615b", "test", 0))
    case x@SignIn(crcId, sid, token, _, _, _, _, _) =>
      println(s"${Platform.currentTime} RECV: $x")
      getAgent(crcId).nextMessage = Some(FriendList(crcId, sid, token, 0, 0, Nil))
    case x@FriendList(crcId, sid, token, _, _, _) =>
      println(s"${Platform.currentTime} RECV: $x")
      getAgent(crcId).nextMessage = Some(Attack(crcId, sid, token, 2, "", 0))
    case x@AbilityList(crcId, sid, token, _) =>
      println(s"${Platform.currentTime} RECV: $x")
      getAgent(crcId).nextMessage = Some(Move(crcId, sid, token, 1, 0))
    case x@Finished(crcId, sid, token, gameOver, _, _, _) if gameOver =>
      println(s"${Platform.currentTime} RECV: $x")
      restartBattle(crcId, sid, token)
    case x@Finished2(crcId, sid, token, gameOver, _, _, _, _) if gameOver =>
      println(s"${Platform.currentTime} RECV: $x")
      restartBattle(crcId, sid, token)
    case x@Move(crcId, sid, token, _, error) if error == 77 =>
      println(s"${Platform.currentTime} RECV: $x")
      restartBattle(crcId, sid, token)
    case x@EnemyName(_, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@StateChanged(_, _, _, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@ScoreChanged(_, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@ThingTaken(_, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@PlayerWounded(_, _, _, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@Move(_, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@UserInfo(_, _, _, _, _, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@Attack(_, _, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@RoundInfo(_, _, _, _, _, _, _, _, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x@FullState(_, _, _, _) => println(s"${Platform.currentTime} RECV: $x")
    case x => println(s"[Worker] Unknown message: $x")
  }

  private def extract(array: Array[Byte], length: Int) = array.toList take length map {t => if (t>=0) t else t+256}

  private def getNextMessage(crcId: Int): Option[Message] = {
    val agent = getAgent(crcId)
    agent.nextMessage map { msg =>
      println(s"${Platform.currentTime} Send: $msg")
      agent.nextMessage = msg match {
        case Move(_, sid, token, 1, _) => Some(Move(crcId, sid, token, 4, 0))
        case Move(_, sid, token, 4, _) => Some(Move(crcId, sid, token, 1, 0))
        case _ => None
      }
      msg
    }
  }

  private def restartBattle(crcId: Int, sid: Int, token: Int): Unit = {
    val agent = getAgent(crcId)
    agent.battlesLeft = max(agent.battlesLeft-1, 0)
    agent.nextMessage = if (agent.battlesLeft > 0) Some(FriendList(crcId, sid, token, 0, 0, Nil)) else None
  }
}

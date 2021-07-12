package ru.mitrakov.self.rush.tester

import scala.annotation.tailrec
import scala.collection.mutable

/**
  * Created by mitrakov on 04.07.2017.
  */
class Agent(var battlesLeft: Int) {
  var nextMessage: Option[Message] = None
}

object Agent {
  private val agents = mutable.Map.empty[Int, Agent]
  var battlesPerAgent = 20

  @tailrec
  def getAgent(crcId: Int): Agent = agents get crcId match {
    case Some(agent) => agent
    case None =>
      agents.put(crcId, new Agent(battlesPerAgent))
      getAgent(crcId)
  }
}
